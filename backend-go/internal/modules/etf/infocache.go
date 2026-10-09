package etf

import (
	"encoding/json"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/loc1263/gohoo"
)

// Caché del «info» de cada ETF (quoteSummary): nombre, tipo, categoría, gastos, fecha de inicio y AUM cambian poco, así que no se
// piden en cada corrida. Precio, volumen y 52W salen siempre del gráfico de 5 años (un request por ETF).
//
// Política de refresco (ver debeRefrescar): cada entrada se renueva cada InfoCicloSemanas semanas, repartidas por ticker para que
// cada corrida semanal renueve ~1/InfoCicloSemanas del universo; nunca antes de InfoEdadMinDias (las corridas repetidas no piden
// nada) y siempre al pasar InfoEdadMaxDias. El AUM puede tener, por tanto, hasta InfoEdadMaxDias días.

const (
	InfoCicloSemanas = 4
	InfoEdadMinDias  = 5
	InfoEdadMaxDias  = 28

	infoCacheArchivo = "info_cache.json"
	infoCacheVersion = 1
)

// camposInfoCache son las claves de etfInfo que se guardan (lo que usan la fila y la decisión de omitir el gráfico).
var camposInfoCache = []string{"quoteType", "longName", "shortName", "netExpenseRatio", "annualReportExpenseRatio", "totalExpenseRatio",
	"expenseRatio", "totalAssets", "fundInceptionDate", "category", "regularMarketPrice", "currentPrice", "previousClose",
	"regularMarketPreviousClose", "volume", "fiftyTwoWeekHigh", "fiftyTwoWeekLow"}

type infoEntry struct {
	FetchedAt time.Time      `json:"fetched_at"`
	Info      map[string]any `json:"info"`
}

type infoCacheFile struct {
	Version int                  `json:"version"`
	Entries map[string]infoEntry `json:"entries"`
}

// infoCache es thread-safe.
type infoCache struct {
	mu      sync.Mutex
	path    string
	entries map[string]infoEntry
	forzar  bool // ignora lo guardado (--refrescar-info)

	Desdecache, Refrescados, Omitidos int
}

// cargarInfoCache lee <dir>/info_cache.json; un archivo ausente, dañado o de otra versión da una caché vacía.
func cargarInfoCache(dir string, forzar bool) *infoCache {
	c := &infoCache{path: filepath.Join(dir, infoCacheArchivo), entries: map[string]infoEntry{}, forzar: forzar}
	b, err := os.ReadFile(c.path)
	if err != nil {
		return c
	}
	var f infoCacheFile
	if json.Unmarshal(b, &f) != nil || f.Version != infoCacheVersion || f.Entries == nil {
		return c
	}
	c.entries = f.Entries
	return c
}

func (c *infoCache) get(ticker string) (infoEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[ticker]
	return e, ok
}

func (c *infoCache) put(ticker string, info etfInfo, now time.Time) {
	sub := map[string]any{}
	for _, k := range camposInfoCache {
		if v, ok := info[k]; ok && v != nil {
			sub[k] = v
		}
	}
	c.mu.Lock()
	c.entries[ticker] = infoEntry{FetchedAt: now, Info: sub}
	c.Refrescados++
	c.mu.Unlock()
}

func (c *infoCache) omitido() {
	c.mu.Lock()
	c.Omitidos++
	c.mu.Unlock()
}

func (c *infoCache) usadoDesdeCache() {
	c.mu.Lock()
	c.Desdecache++
	c.mu.Unlock()
}

// debeRefrescar decide si hay que volver a pedir el info de un ticker que ya está en caché.
func (c *infoCache) debeRefrescar(ticker string, e infoEntry, now time.Time) bool {
	if c.forzar {
		return true
	}
	edad := now.Sub(e.FetchedAt)
	if edad >= InfoEdadMaxDias*24*time.Hour {
		return true
	}
	if edad < InfoEdadMinDias*24*time.Hour {
		return false
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(ticker))
	semana := now.Unix() / (7 * 24 * 3600)
	return (int64(h.Sum32())+semana)%InfoCicloSemanas == 0
}

// guardar escribe la caché de forma atómica, conservando solo los tickers de la lista actual.
func (c *infoCache) guardar(tickers []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	vivos := make(map[string]infoEntry, len(tickers))
	for _, t := range tickers {
		if e, ok := c.entries[t]; ok {
			vivos[t] = e
		}
	}
	b, err := json.Marshal(infoCacheFile{Version: infoCacheVersion, Entries: vivos})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}

// categoriaExcluida: categorías que el ranking descarta (ranking.go).
func categoriaExcluida(cat string) bool { return inList(cat, CategoriasExcluirRanking) }

// omitirHistorial dice si el ETF será descartado por el ranking de todos modos —no es ETF, no tiene AUM, tiene menos de LiqAUMMin o
// su categoría está excluida— y por tanto no hace falta pedir su gráfico.
func omitirHistorial(info etfInfo, m Meta) bool {
	if tipo, _ := info["quoteType"].(string); tipo != "ETF" {
		return true
	}
	aum, ok := gohoo.AsFloat(info["totalAssets"])
	if !ok || math.IsNaN(aum) || aum < LiqAUMMin {
		return true
	}
	cat := m.Category
	if cat == "" {
		cat, _ = info["category"].(string)
	}
	return categoriaExcluida(cat)
}

// datosGrafico son el precio, el volumen de la última sesión completa y el rango de 52 semanas derivados del gráfico diario.
type datosGrafico struct {
	Precio, Volumen, Max52, Min52 float64
	HayVolumen                    bool
}

// derivarDeGrafico calcula, a partir de las barras diarias SIN ajustar (Close/High/Low crudos):
//   - Precio: el último cierre (la vela de hoy, si el mercado está abierto, trae el precio actual).
//   - Volumen: el de la última vela con fecha anterior a hoy (hora del exchange): la sesión completa más reciente. El volumen de
//     «info» es el de la sesión en curso y deja el universo depurado dependiendo de la hora de la corrida.
//   - 52W: máximo de High y mínimo de Low desde hace un año (igual que fiftyTwoWeekHigh/Low de Yahoo; verificado en 15 ETFs).
func derivarDeGrafico(bars []gohoo.Bar, now time.Time) (datosGrafico, bool) {
	if len(bars) == 0 {
		return datosGrafico{}, false
	}
	last := bars[len(bars)-1]
	loc := last.Time.Location()
	hoy := now.In(loc).Format("2006-01-02")
	corte := now.In(loc).AddDate(-1, 0, 0)
	d := datosGrafico{Precio: last.Close, Max52: math.Inf(-1), Min52: math.Inf(1)}
	for i := len(bars) - 1; i >= 0; i-- {
		b := bars[i]
		if !d.HayVolumen && b.Time.Format("2006-01-02") < hoy && !math.IsNaN(b.Volume) {
			d.Volumen, d.HayVolumen = b.Volume, true
		}
		if b.Time.Before(corte) {
			continue
		}
		if !math.IsNaN(b.High) {
			d.Max52 = math.Max(d.Max52, b.High)
		}
		if !math.IsNaN(b.Low) {
			d.Min52 = math.Min(d.Min52, b.Low)
		}
	}
	if math.IsInf(d.Max52, 0) {
		d.Max52 = math.NaN()
	}
	if math.IsInf(d.Min52, 0) {
		d.Min52 = math.NaN()
	}
	return d, !math.IsNaN(d.Precio)
}

// volumenFila: el de la última sesión completa del gráfico; en filas sin gráfico, el del info guardado.
func volumenFila(v *validado) any {
	if v.graf != nil {
		if !v.graf.HayVolumen {
			return naValue
		}
		return intIfWhole(v.graf.Volumen)
	}
	return intIfWhole(infoVal(v.info, "volume"))
}

// rango52 es el máximo (alto=true) o mínimo de 52 semanas: del gráfico, redondeado a 4 decimales para quitar el ruido de
// coma flotante de Yahoo; en filas sin gráfico, el del info guardado.
func rango52(v *validado, info etfInfo, alto bool) any {
	if v.graf == nil {
		if alto {
			return infoVal(info, "fiftyTwoWeekHigh")
		}
		return infoVal(info, "fiftyTwoWeekLow")
	}
	x := v.graf.Min52
	if alto {
		x = v.graf.Max52
	}
	if math.IsNaN(x) {
		return naValue
	}
	return round(x, 4)
}
