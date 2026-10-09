package etf

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/loc1263/gohoo"

	"fogradar/internal/quant"
)

var ny = time.FixedZone("EDT", -4*3600)

// barras diarias hábiles terminando en `fin` (la última es «hoy», con volumen parcial).
func barras(fin time.Time, n int, precio0 float64) []gohoo.Bar {
	var out []gohoo.Bar
	d := time.Date(fin.Year(), fin.Month(), fin.Day(), 9, 30, 0, 0, ny)
	var rev []gohoo.Bar
	for len(rev) < n {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			rev = append(rev, gohoo.Bar{Time: d})
		}
		d = d.AddDate(0, 0, -1)
	}
	for i := len(rev) - 1; i >= 0; i-- {
		k := float64(len(rev) - 1 - i)
		p := precio0 + k*0.01
		b := rev[i]
		b.Open, b.High, b.Low, b.Close, b.AdjClose, b.Volume = p, p+1, p-1, p, p*0.99, 100000
		out = append(out, b)
	}
	out[len(out)-1].Volume = 5000 // sesión en curso: parcial
	return out
}

type fakeSrc struct {
	qs, charts atomic.Int32
	aum        map[string]float64
	now        time.Time
}

func (f *fakeSrc) Chart(_ context.Context, t string, _ gohoo.ChartParams) (*gohoo.ChartResult, error) {
	f.charts.Add(1)
	return &gohoo.ChartResult{Bars: barras(f.now, 1500, 50)}, nil
}

func (f *fakeSrc) QuoteSummary(_ context.Context, t string, _ []string) (map[string]any, error) {
	f.qs.Add(1)
	aum := f.aum[t]
	return map[string]any{
		"price":         map[string]any{"regularMarketPrice": map[string]any{"raw": 55.0}, "longName": t + " Fund", "quoteType": "ETF"},
		"summaryDetail": map[string]any{"totalAssets": map[string]any{"raw": aum}, "volume": map[string]any{"raw": 5000.0}, "fiftyTwoWeekHigh": map[string]any{"raw": 1.0}},
		"fundProfile":   map[string]any{"categoryName": "Large Blend"},
	}, nil
}

func (f *fakeSrc) Fundamentals(context.Context, string, []string) (map[string]any, error) {
	return nil, nil
}

func TestDerivarDeGrafico(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC) // viernes
	bars := barras(now, 400, 50)
	d, ok := derivarDeGrafico(bars, now)
	if !ok || d.Precio != bars[len(bars)-1].Close {
		t.Fatalf("precio: %+v", d)
	}
	if !d.HayVolumen || d.Volumen != 100000 { // el de ayer, no el parcial de hoy (5000)
		t.Errorf("volumen: %+v", d)
	}
	if d.Max52 != bars[len(bars)-1].High || d.Min52 >= d.Max52 {
		t.Errorf("52W: %+v", d)
	}
	// Corrida un sábado: la última vela es el viernes y cuenta como sesión completa.
	sab := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	d2, _ := derivarDeGrafico(bars, sab)
	if d2.Volumen != 5000 {
		t.Errorf("sábado debía usar el volumen del viernes: %+v", d2)
	}
	if _, ok := derivarDeGrafico(nil, now); ok {
		t.Error("sin barras debía fallar")
	}
}

func TestOmitirHistorial(t *testing.T) {
	etf := func(aum any, cat string) etfInfo {
		i := etfInfo{"quoteType": "ETF", "category": cat}
		if aum != nil {
			i["totalAssets"] = aum
		}
		return i
	}
	casos := []struct {
		nombre string
		info   etfInfo
		meta   Meta
		want   bool
	}{
		{"liquido", etf(5e8, "Large Blend"), Meta{}, false},
		{"AUM bajo", etf(1e7, "Large Blend"), Meta{}, true},
		{"AUM justo en el umbral", etf(float64(LiqAUMMin), "Large Blend"), Meta{}, false},
		{"sin AUM", etf(nil, "Large Blend"), Meta{}, true},
		{"categoría excluida (info)", etf(5e8, "Target Maturity"), Meta{}, true},
		{"categoría excluida (lista manda)", etf(5e8, "Large Blend"), Meta{Category: "Ultrashort Bond"}, true},
		{"lista pisa al info", etf(5e8, "Target Maturity"), Meta{Category: "Large Blend"}, false},
		{"no es ETF", etfInfo{"quoteType": "EQUITY", "totalAssets": 5e8}, Meta{}, true},
	}
	for _, c := range casos {
		if got := omitirHistorial(c.info, c.meta); got != c.want {
			t.Errorf("%s: %v", c.nombre, got)
		}
	}
}

func TestDebeRefrescar(t *testing.T) {
	c := &infoCache{}
	base := time.Date(2026, 10, 12, 6, 0, 0, 0, time.UTC)
	at := func(dias int) infoEntry { return infoEntry{FetchedAt: base.AddDate(0, 0, -dias)} }
	if c.debeRefrescar("SPY", at(2), base) {
		t.Error("menos de 5 días nunca refresca")
	}
	if !c.debeRefrescar("SPY", at(InfoEdadMaxDias), base) {
		t.Error("al llegar al máximo siempre refresca")
	}
	// A 7 días: cada ticker toca una de cada InfoCicloSemanas semanas.
	n, tocan := 400, 0
	for i := 0; i < n; i++ {
		if c.debeRefrescar("T"+string(rune('A'+i%26))+string(rune('a'+i/26)), at(7), base) {
			tocan++
		}
	}
	if tocan < n/InfoCicloSemanas/2 || tocan > n*2/InfoCicloSemanas {
		t.Errorf("refrescan %d de %d (esperado ≈ 1/%d)", tocan, n, InfoCicloSemanas)
	}
	c.forzar = true
	if !c.debeRefrescar("SPY", at(0), base) {
		t.Error("forzar siempre refresca")
	}
}

func TestProcesarETFsUsaCache(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC)
	lista := []byte("Ticker;Name;Company;Category\nAAA;Aaa;;\nBBB;Bbb;;\nCCC;Ccc;;\n")
	f := &fakeSrc{now: now, aum: map[string]float64{"AAA": 5e8, "BBB": 1e7, "CCC": 9e8}} // BBB: AUM bajo
	o := ScreenerOpts{OutputDir: dir, ListaCSV: lista, Concurrencia: 2, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return now }}

	res, err := ProcesarETFs(context.Background(), f, o)
	if err != nil || res == nil || res.NValidos != 3 {
		t.Fatalf("1ª corrida: %v %+v", err, res)
	}
	// 1ª: mercado (^GSPC) + 3 quoteSummary + 2 gráficos (BBB se omite).
	if f.qs.Load() != 3 || f.charts.Load() != 1+2 {
		t.Fatalf("1ª corrida: qs=%d charts=%d", f.qs.Load(), f.charts.Load())
	}
	df, err := quant.ReadCSVFile(res.CSV)
	if err != nil {
		t.Fatal(err)
	}
	tk, vol, hi, perf := df.Col("Ticker"), df.Col("Volumen"), df.Col("52W High"), df.Col("Performance 1Y (%)")
	for i := 0; i < df.N; i++ {
		switch tk.Cell(i) {
		case "AAA":
			if vol.Floats()[i] != 100000 || hi.Floats()[i] <= 50 || perf.IsNull(i) {
				t.Errorf("AAA: vol=%v hi=%v", vol.Floats()[i], hi.Floats()[i])
			}
		case "BBB":
			if !perf.IsNull(i) { // sin gráfico: rendimientos en N/A
				t.Errorf("BBB debía quedar sin rendimientos")
			}
		}
	}
	if _, err := os.Stat(filepath.Join(dir, infoCacheArchivo)); err != nil {
		t.Fatal("no guardó la caché:", err)
	}

	// 2ª corrida al día siguiente: solo los gráficos de AAA, CCC y ^GSPC; ningún quoteSummary.
	f2 := &fakeSrc{now: now, aum: f.aum}
	o.Now = func() time.Time { return now.Add(24 * time.Hour) }
	if _, err := ProcesarETFs(context.Background(), f2, o); err != nil {
		t.Fatal(err)
	}
	if f2.qs.Load() != 0 || f2.charts.Load() != 1+2 {
		t.Errorf("2ª corrida: qs=%d charts=%d", f2.qs.Load(), f2.charts.Load())
	}
	// --refrescar-info vuelve a pedirlo todo.
	f3 := &fakeSrc{now: now, aum: f.aum}
	o.RefrescarInfo = true
	if _, err := ProcesarETFs(context.Background(), f3, o); err != nil {
		t.Fatal(err)
	}
	if f3.qs.Load() != 3 {
		t.Errorf("3ª corrida: qs=%d", f3.qs.Load())
	}
	b, _ := os.ReadFile(filepath.Join(dir, infoCacheArchivo))
	if !strings.Contains(string(b), `"AAA"`) {
		t.Error("la caché no tiene AAA")
	}
}
