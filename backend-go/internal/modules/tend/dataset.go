package tend

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"fogradar/internal/fx"
	"fogradar/internal/quant"
	"fogradar/internal/yahoo"
)

// Construcción del dataset consolidado a partir de los CSV crudos:
//   - convierte a USD los tickers que cotizan en moneda local (core.fx, con subunidades GBp/ZAc/ILA);
//   - alinea las series en un calendario semanal común (semanas que terminan en domingo);
//   - filtra tickers con historia insuficiente (< MIN_WEEKS_REQUIRED);
//   - determina la tasa libre de riesgo: macro_data (Treasury 3M) > ^IRX > 4 %.

// Excluido es un ticker que quedó fuera del dataset, con su motivo.
type Excluido struct{ Ticker, Nombre, Motivo string }

// CurrencyFX son los pares de FX fijos (tienen prioridad); el resto de monedas usa USD<XXX>=X.
var CurrencyFX = map[string]string{"KRW": "USDKRW=X", "HKD": "USDHKD=X", "CNY": "USDCNY=X", "JPY": "USDJPY=X", "INR": "USDINR=X", "TWD": "USDTWD=X"}

// FXTicker devuelve el par de Yahoo de una moneda ("USD" para USD; "" si no es una moneda válida).
func FXTicker(currency string) string {
	if currency == "USD" {
		return "USD"
	}
	if t, ok := CurrencyFX[currency]; ok {
		return t
	}
	iso, _, ok := fx.Normalize(currency)
	if !ok {
		return ""
	}
	if t, ok := CurrencyFX[iso]; ok {
		return t
	}
	return fx.USDPair(iso)
}

// weekEnd lleva una fecha al domingo de su semana (período W-SUN), a medianoche UTC.
func weekEnd(t time.Time) time.Time {
	t = t.UTC()
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return d.AddDate(0, 0, (7-int(d.Weekday()))%7)
}

// loadPriceCSV lee la columna Close de un CSV de precios en una serie semanal (cierre de la semana, sin duplicados, ordenada).
func loadPriceCSV(path string) (fx.Series, bool) {
	bars, err := yahoo.ReadBarsCSV(path)
	if err != nil || len(bars) == 0 {
		return fx.Series{}, false
	}
	byWeek := map[time.Time]float64{}
	for _, b := range bars {
		byWeek[weekEnd(b.Time)] = b.Close // en fechas repetidas gana la última (keep="last")
	}
	var s fx.Series
	for t := range byWeek {
		s.T = append(s.T, t)
	}
	sort.Slice(s.T, func(i, j int) bool { return s.T[i].Before(s.T[j]) })
	for _, t := range s.T {
		s.V = append(s.V, byWeek[t])
	}
	return s, true
}

func countValid(v []float64) int { return len(quant.DropNaN(v)) }

// BuildPriceMatrix arma la matriz de precios en USD; devuelve también los tickers excluidos. Lanza error si no queda ninguno.
func BuildPriceMatrix(cacheDir string, u *Universe, p Params, warn func(string, ...any)) (*Prices, []Excluido, error) {
	rawPrices, rawFX := filepath.Join(cacheDir, "prices"), filepath.Join(cacheDir, "fx")
	// Solo se avisa por monedas que el universo ACTUAL necesita.
	needed := map[string]bool{}
	for _, t := range u.All() {
		if c := u.Info[t].Currency; c != "USD" {
			needed[c] = true
		}
	}
	var currs []string
	for c := range needed {
		currs = append(currs, c)
	}
	sort.Strings(currs)
	fxRates := map[string]fx.Series{}
	for _, c := range currs {
		ft := FXTicker(c)
		if ft == "" {
			warn("Falta el par FX para %s; los tickers en %s se excluirán si no se resuelve.", c, c)
			continue
		}
		path := yahoo.CachePath(rawFX, ft)
		if _, err := os.Stat(path); err != nil {
			warn("Falta el par FX para %s (%s); los tickers en %s se excluirán si no se resuelve.", c, ft, c)
			continue
		}
		if s, ok := loadPriceCSV(path); ok {
			fxRates[c] = s
		}
	}

	type col struct {
		t string
		s fx.Series
	}
	var cols []col
	var excluded []Excluido
	for _, t := range u.All() {
		info := u.Info[t]
		path := yahoo.CachePath(rawPrices, t)
		if _, err := os.Stat(path); err != nil {
			warn("Sin datos descargados para %s (%s); se excluye.", t, info.Name)
			excluded = append(excluded, Excluido{t, info.Name, "sin datos descargados"})
			continue
		}
		s, ok := loadPriceCSV(path)
		if !ok {
			excluded = append(excluded, Excluido{t, info.Name, "csv vacío o sin columna Close"})
			continue
		}
		if info.Currency != "USD" {
			rate, ok := fxRates[info.Currency]
			if !ok {
				excluded = append(excluded, Excluido{t, info.Name, "falta FX para " + info.Currency})
				continue
			}
			s = fx.ToUSD(s, rate, info.Currency) // precio_local / (moneda local por USD)
		}
		if n := countValid(s.V); n < p.MinWeeks {
			excluded = append(excluded, Excluido{t, info.Name, fmt.Sprintf("historia insuficiente (%d semanas)", n)})
			continue
		}
		cols = append(cols, col{t, s})
	}
	if len(cols) == 0 {
		return nil, excluded, fmt.Errorf("no se pudo construir ninguna serie de precios en USD: verifica que la descarga de precios y FX haya funcionado")
	}

	// unión de fechas + forward-fill acotado (máx. 2 semanas)
	set := map[time.Time]bool{}
	for _, c := range cols {
		for _, t := range c.s.T {
			set[t] = true
		}
	}
	out := &Prices{}
	for t := range set {
		out.Dates = append(out.Dates, t)
	}
	sort.Slice(out.Dates, func(i, j int) bool { return out.Dates[i].Before(out.Dates[j]) })
	pos := make(map[time.Time]int, len(out.Dates))
	for i, t := range out.Dates {
		pos[t] = i
	}
	for _, c := range cols {
		v := make([]float64, len(out.Dates))
		for i := range v {
			v[i] = math.NaN()
		}
		for i, t := range c.s.T {
			v[pos[t]] = c.s.V[i]
		}
		out.Tickers = append(out.Tickers, c.t)
		out.Cols = append(out.Cols, v)
	}
	out.Cols = ffillLimit(out)
	return out, excluded, nil
}

// ffillLimit repite el último valor hasta 2 semanas seguidas de hueco (ffill(limit=2)); los huecos iniciales quedan NaN.
func ffillLimit(p *Prices) [][]float64 {
	out := make([][]float64, len(p.Cols))
	for j, src := range p.Cols {
		v := make([]float64, len(src))
		copy(v, src)
		run := 0
		for i := 1; i < len(v); i++ {
			if math.IsNaN(v[i]) {
				if !math.IsNaN(v[i-1]) && run < 2 {
					v[i] = v[i-1]
					run++
				}
			} else {
				run = 0
			}
		}
		out[j] = v
	}
	return out
}

// DetermineRiskFreeRate: macro_data (Treasury 3M) > ^IRX > 4 %.
func DetermineRiskFreeRate(cacheDir string, warn func(string, ...any)) float64 {
	if mf := LatestMacroFile(cacheDir); mf != "" {
		if df, err := quant.ReadCSVFile(mf); err == nil {
			ind, val := df.Col("Indicador"), df.Col("Último valor")
			if ind != nil && val != nil {
				for i := 0; i < df.N; i++ {
					if !ind.IsNull(i) && strings.Contains(ind.Cell(i), "Treasury 3M") {
						return val.At(i) / 100.0
					}
				}
			}
		} else {
			warn("No se pudo leer tasa libre de riesgo de %s: %v", mf, err)
		}
	}
	if s, ok := loadPriceCSV(yahoo.CachePath(filepath.Join(cacheDir, "prices"), "^IRX")); ok {
		if v := quant.DropNaN(s.V); len(v) > 0 {
			return v[len(v)-1] / 100.0
		}
	}
	warn("No se encontró tasa libre de riesgo; se usa 4.0% por defecto.")
	return 0.04
}

// BuildDataset lee las series de cacheDir y escribe en procDir (la carpeta processed/ de la corrida) prices_usd.csv,
// weekly_returns.csv, risk_free_rate.txt y excluded_tickers.txt.
func BuildDataset(cacheDir, procDir string, u *Universe, p Params, warn func(string, ...any)) (*Prices, float64, error) {
	prices, excluded, err := BuildPriceMatrix(cacheDir, u, p, warn)
	if err != nil {
		return nil, 0, err
	}
	dir := procDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, 0, err
	}
	pf := pricesFrame(prices)
	if err := writeFrameFile(filepath.Join(dir, "prices_usd.csv"), pf); err != nil {
		return nil, 0, err
	}
	rets := WeeklyReturns(prices)
	if err := writeFrameFile(filepath.Join(dir, "weekly_returns.csv"), pricesFrame(rets)); err != nil {
		return nil, 0, err
	}
	rf := DetermineRiskFreeRate(cacheDir, warn)
	if err := os.WriteFile(filepath.Join(dir, "risk_free_rate.txt"), []byte(quant.FloatRepr(rf)), 0o644); err != nil {
		return nil, 0, err
	}
	var sb strings.Builder
	for _, e := range excluded {
		sb.WriteString(e.Ticker + "\t" + e.Nombre + "\t" + e.Motivo + "\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "excluded_tickers.txt"), []byte(sb.String()), 0o644); err != nil {
		return nil, 0, err
	}
	warn("Dataset construido: %d tickers, %d semanas; excluidos: %d; tasa libre de riesgo: %s", len(prices.Tickers), len(prices.Dates), len(excluded), strconv.FormatFloat(rf*100, 'f', 4, 64)+"%")
	return prices, rf, nil
}

// pricesFrame convierte la matriz a un Frame con la fecha como primera columna («Date»).
func pricesFrame(p *Prices) *quant.Frame {
	f := quant.NewFrame(len(p.Dates))
	dates := make([]string, len(p.Dates))
	for i, d := range p.Dates {
		dates[i] = d.Format("2006-01-02")
	}
	f.Set(strCol("Date", dates))
	for j, t := range p.Tickers {
		f.Set(fCol(t, p.Cols[j]))
	}
	return f
}

// WeeklyReturns es prices.pct_change().dropna(how="all"): pct_change rellena hacia adelante los faltantes.
func WeeklyReturns(prices *Prices) *Prices {
	rets := &Prices{Tickers: prices.Tickers, Cols: make([][]float64, len(prices.Cols))}
	r := make([][]float64, len(prices.Cols))
	for j, c := range prices.Cols {
		v := append([]float64(nil), c...)
		for i := 1; i < len(v); i++ {
			if math.IsNaN(v[i]) {
				v[i] = v[i-1]
			}
		}
		r[j] = quant.PctChangeNaN(v)
	}
	for i, d := range prices.Dates {
		all := true
		for j := range r {
			if !math.IsNaN(r[j][i]) {
				all = false
				break
			}
		}
		if all {
			continue
		}
		rets.Dates = append(rets.Dates, d)
		for j := range r {
			rets.Cols[j] = append(rets.Cols[j], r[j][i])
		}
	}
	return rets
}
