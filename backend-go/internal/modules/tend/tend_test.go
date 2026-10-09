package tend

import (
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const fixtures = "../../../../backend/tests/tendencia/fixtures"

func gunzip(t *testing.T, src, dst string) {
	t.Helper()
	f, err := os.Open(src)
	if err != nil {
		t.Skipf("sin fixtures: %v", err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(zr)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func close7(a, b string) bool {
	if a == b {
		return true
	}
	x, e1 := strconv.ParseFloat(a, 64)
	y, e2 := strconv.ParseFloat(b, 64)
	if e1 != nil || e2 != nil {
		return false
	}
	return math.Abs(x-y) <= 1e-9+1e-7*math.Max(math.Abs(x), math.Abs(y))
}

// compareCSV compara por clave (primera columna del archivo cuando es única, o la columna ticker): el orden entre
// empates (pesos en 0) es inestable en pandas/numpy y no es reproducible (documentado en PARIDAD.md).
func compareCSV(t *testing.T, name, got, want string, skip map[string]bool) {
	t.Helper()
	g, w := readCSV(t, got), readCSV(t, want)
	if len(g) != len(w) {
		t.Errorf("%s: %d filas, esperadas %d", name, len(g), len(w))
		return
	}
	key := 0
	for j, h := range w[0] {
		if h == "ticker" {
			key = j
		}
	}
	idx := map[string][]string{}
	for _, r := range g[1:] {
		idx[r[key]] = r
	}
	for _, wr := range w[1:] {
		gr, ok := idx[wr[key]]
		if !ok {
			t.Errorf("%s: falta %q", name, wr[key])
			continue
		}
		if len(gr) != len(wr) {
			t.Errorf("%s %q: %d columnas, esperadas %d", name, wr[key], len(gr), len(wr))
			continue
		}
		for j := range wr {
			if skip[w[0][j]] || w[0][j] == "rank" {
				continue
			}
			if !close7(gr[j], wr[j]) {
				t.Errorf("%s %q col %s: %q != %q", name, wr[key], w[0][j], gr[j], wr[j])
			}
		}
	}
}

func TestParidadModelado(t *testing.T) {
	root := t.TempDir()
	mod := filepath.Join(root, "data")
	gunzip(t, filepath.Join(fixtures, "data/processed/prices_usd.csv.gz"), filepath.Join(mod, "processed/prices_usd.csv"))
	for _, f := range []string{"processed/risk_free_rate.txt", "macro/macro_data_20260925.csv", "raw/prices/_VIX.csv", "universe/universe_20260926.json"} {
		b, err := os.ReadFile(filepath.Join(fixtures, "data", f))
		if err != nil {
			t.Fatal(err)
		}
		_ = os.MkdirAll(filepath.Dir(filepath.Join(mod, f)), 0o755)
		_ = os.WriteFile(filepath.Join(mod, f), b, 0o644)
	}
	prices, err := LoadPricesUSD(mod)
	if err != nil {
		t.Fatal(err)
	}
	rf, err := LoadRiskFree(mod)
	if err != nil {
		t.Fatal(err)
	}
	u := LoadUniverse(mod, time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC))
	p := DefaultParams()
	run := filepath.Join(root, "run")
	_ = os.MkdirAll(run, 0o755)
	warn := func(f string, a ...any) { t.Logf(f, a...) }

	for _, spec := range Analyses(u, p) {
		if _, err := RunSelection(prices, rf, run, u, spec, p, warn); err != nil {
			t.Fatalf("%s: %v", spec.Label, err)
		}
	}
	if _, err := RunForecast(prices, run, mod, u, p, warn); err != nil {
		t.Fatal(err)
	}
	if err := RunMomentum(prices, run, u, p, warn); err != nil {
		t.Fatal(err)
	}

	exp := filepath.Join(fixtures, "expected/20260926_01")
	ents, _ := os.ReadDir(exp)
	skip := map[string]bool{"precio_proyectado_6m_usd": true, "precio_min_90pct_6m_usd": true, "precio_max_90pct_6m_usd": true, "retorno_esperado_6m_pct": true}
	n := 0
	for _, e := range ents {
		name := e.Name()
		if filepath.Ext(name) != ".csv" {
			continue
		}
		n++
		compareCSV(t, name, filepath.Join(run, name), filepath.Join(exp, name), skip)
	}
	if n == 0 {
		t.Fatal("sin CSV esperados")
	}
	pdfPath, err := GenerarPDF(run, u, p, true, time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(pdfPath); !strings.HasPrefix(string(b), "%PDF-") {
		t.Errorf("PDF inválido: %s", pdfPath)
	} else if out := os.Getenv("TD_PDF_OUT"); out != "" {
		_ = os.WriteFile(out, b, 0o644)
	}
	payload, err := BuildDashboardPayload(run, u, p, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var gotJ, wantJ any
	if err := json.Unmarshal(PyJSON(payload), &gotJ); err != nil {
		t.Fatal(err)
	}
	gunzip(t, filepath.Join(exp, "dashboard_data.json.gz"), filepath.Join(root, "dash.json"))
	wb, _ := os.ReadFile(filepath.Join(root, "dash.json"))
	if err := json.Unmarshal(wb, &wantJ); err != nil {
		t.Fatal(err)
	}
	corr := map[string]bool{"generated_at": true, "run_dir": true, "precio_proyectado_6m_usd": true, "precio_min_90pct_6m_usd": true,
		"precio_max_90pct_6m_usd": true, "retorno_esperado_6m_pct": true, "retorno_6m_prom": true}
	jsonDiff(t, "$", gotJ, wantJ, corr)

	got, _ := os.ReadFile(filepath.Join(run, "market_regime_notes.txt"))
	want, _ := os.ReadFile(filepath.Join(exp, "market_regime_notes.txt"))
	if strings.ReplaceAll(string(got), "", "") != strings.ReplaceAll(string(want), "", "") {
		t.Errorf("notas de régimen:\n%s\n---\n%s", got, want)
	}
}

func jsonDiff(t *testing.T, path string, got, want any, skip map[string]bool) {
	t.Helper()
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok || len(g) != len(w) {
			t.Errorf("%s: objeto distinto (%d vs %d claves)", path, len(g), len(w))
			return
		}
		for k, wv := range w {
			if skip[k] {
				continue
			}
			jsonDiff(t, path+"."+k, g[k], wv, skip)
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			t.Errorf("%s: lista distinta (%d vs %d)", path, len(g), len(w))
			return
		}
		for i := range w {
			jsonDiff(t, path+"["+strconv.Itoa(i)+"]", g[i], w[i], skip)
		}
	case float64:
		g, ok := got.(float64)
		if !ok || math.Abs(g-w) > 1e-9+1e-7*math.Abs(w) {
			t.Errorf("%s: %v != %v", path, got, want)
		}
	default:
		if got != want {
			t.Errorf("%s: %v != %v", path, got, want)
		}
	}
}

func TestWeeklyReturns(t *testing.T) {
	root := t.TempDir()
	gunzip(t, filepath.Join(fixtures, "data/processed/prices_usd.csv.gz"), filepath.Join(root, "prices_usd.csv"))
	gunzip(t, filepath.Join(fixtures, "data/processed/weekly_returns.csv.gz"), filepath.Join(root, "want.csv"))
	prices, err := ReadPricesUSD(filepath.Join(root, "prices_usd.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFrameFile(filepath.Join(root, "got.csv"), pricesFrame(WeeklyReturns(prices))); err != nil {
		t.Fatal(err)
	}
	compareCSV(t, "weekly_returns", filepath.Join(root, "got.csv"), filepath.Join(root, "want.csv"), nil)
	if err := writeFrameFile(filepath.Join(root, "gp.csv"), pricesFrame(prices)); err != nil {
		t.Fatal(err)
	}
	compareCSV(t, "prices_usd", filepath.Join(root, "gp.csv"), filepath.Join(root, "prices_usd.csv"), nil)
}
