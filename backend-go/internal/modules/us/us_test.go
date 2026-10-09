package us

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"fogradar/internal/core"
)

const runDir = "testdata/runs/2026-09-25T21-00-01Z"

func loadRun(t *testing.T) ([]CandidateRow, PriceMatrix) {
	t.Helper()
	var c RunResponse
	if e := ReadRunFile(runDir, "candidates.json", &c); e != nil {
		t.Fatal(e)
	}
	var m PriceMatrix
	if e := ReadRunFile(runDir, "prices.json", &m); e != nil {
		t.Fatal(e)
	}
	return c.Rows, m
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol*math.Max(1, math.Abs(b)) }

func ptrNear(t *testing.T, what string, got, want *float64, tol float64) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Errorf("%s: %v (go) vs %v (py)", what, got, want)
	case !near(*got, *want, tol):
		t.Errorf("%s: %v (go) vs %v (py)", what, *got, *want)
	}
}

// Paridad: evaluate_score del original produce exactamente el score guardado de cada candidato.
func TestScoreIdenticoParaTodosLosCandidatos(t *testing.T) {
	rows, _ := loadRun(t)
	if len(rows) < 40 {
		t.Fatalf("fixture con %d candidatos", len(rows))
	}
	for _, r := range rows {
		p := r.Price
		got := EvaluateScore(Evaluation{Price: &p, MarketCap: r.MarketCap, Valuation: r.Valuation, Profitability: r.Profitability,
			FinancialHealth: r.FinancialHealth, ShareholderReturn: r.ShareholderReturn, Technical: r.Technical})
		if !reflect.DeepEqual(got, r.Score) {
			gj, _ := json.Marshal(got)
			wj, _ := json.Marshal(r.Score)
			t.Errorf("%s:\n go: %s\n py: %s", r.Ticker, gj, wj)
		}
	}
}

func seriesDe(t *testing.T, m PriceMatrix, ticker string) ([]time.Time, []float64) {
	t.Helper()
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	var ts []time.Time
	var cs []float64
	for i, d := range m.Dates {
		tm, err := time.ParseInLocation("2006-01-02", d, ny)
		if err != nil {
			t.Fatal(err)
		}
		ts = append(ts, tm)
		if v := m.Prices[ticker][i]; v != nil {
			cs = append(cs, *v)
		} else {
			cs = append(cs, math.NaN())
		}
	}
	return ts, cs
}

func TestMomentumYRetornosDesdePricesJSON(t *testing.T) {
	rows, m := loadRun(t)
	for _, r := range rows {
		ts, cs := seriesDe(t, m, r.Ticker)
		got := ComputeMomentum(ts, cs)
		if got == nil {
			t.Fatalf("%s: sin momentum", r.Ticker)
		}
		if !near(got.Price, r.Price, 1e-9) || !near(got.Momentum121, r.Momentum121, 1e-9) {
			t.Errorf("%s: price/momentum %v %v vs %v %v", r.Ticker, got.Price, got.Momentum121, r.Price, r.Momentum121)
		}
		ptrNear(t, r.Ticker+" 1d", got.Return1d, r.Return1d, 1e-9)
		ptrNear(t, r.Ticker+" 5d", got.Return5d, r.Return5d, 1e-9)
		ptrNear(t, r.Ticker+" 3m", got.Return3m, r.Return3m, 1e-9)
		ptrNear(t, r.Ticker+" 6m", got.Return6m, r.Return6m, 1e-9)
		ptrNear(t, r.Ticker+" 1y", got.Return1y, r.Return1y, 1e-9)
		ptrNear(t, r.Ticker+" ytd", got.ReturnYtd, r.ReturnYtd, 1e-9)
	}
}

func TestIndicadoresTecnicosDesdePricesJSON(t *testing.T) {
	rows, m := loadRun(t)
	for _, r := range rows {
		_, cs := seriesDe(t, m, r.Ticker)
		var clean []float64
		for _, c := range cs {
			if !math.IsNaN(c) {
				clean = append(clean, c)
			}
		}
		tech := BuildTechnical(clean, nil)
		ptrNear(t, r.Ticker+" sma50", tech.Sma50, r.Technical.Sma50, 1e-9)
		ptrNear(t, r.Ticker+" sma200", tech.Sma200, r.Technical.Sma200, 1e-9)
		ptrNear(t, r.Ticker+" rsi", tech.Rsi14, r.Technical.Rsi14, 1e-7)
		if (tech.Macd.V == nil) != (r.Technical.Macd.V == nil) {
			t.Fatalf("%s: macd presente en uno solo", r.Ticker)
		}
		if tech.Macd.V != nil {
			for k, pair := range map[string][2]float64{"macd": {tech.Macd.V.Macd, r.Technical.Macd.V.Macd},
				"signal": {tech.Macd.V.Signal, r.Technical.Macd.V.Signal}, "hist": {tech.Macd.V.Histogram, r.Technical.Macd.V.Histogram}} {
				if !near(pair[0], pair[1], 1e-7) {
					t.Errorf("%s %s: %v vs %v", r.Ticker, k, pair[0], pair[1])
				}
			}
		}
	}
}

type expected struct {
	Tickers []string `json:"tickers"`
	BL      struct {
		Prior     map[string]float64 `json:"prior_returns"`
		Views     map[string]float64 `json:"views"`
		Posterior map[string]float64 `json:"posterior_returns"`
	} `json:"black_litterman"`
}

func loadExpected(t *testing.T) expected {
	t.Helper()
	b, err := os.ReadFile("testdata/portfolio_expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var e expected
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	return e
}

func capsScores(rows []CandidateRow) (map[string]*float64, map[string]*float64) {
	caps, scores := map[string]*float64{}, map[string]*float64{}
	for _, r := range rows {
		caps[r.Ticker], scores[r.Ticker] = r.MarketCap, r.Score.Overall
	}
	return caps, scores
}

// Black-Litterman contra el portfolio.py original. Corrección de la Fase 7 (docs/PARIDAD.md): π, Q y el
// posterior son retornos totales (exceso + rf), así que se desplazan exactamente rf respecto del original.
func TestBlackLittermanContraElOriginal(t *testing.T) {
	rows, _ := loadRun(t)
	exp := loadExpected(t)
	r, err := LoadPriceMatrix(runDir, exp.Tickers)
	if err != nil {
		t.Fatal(err)
	}
	caps, scores := capsScores(rows)
	got, err := BlackLittermanWeights(r, caps, scores, DefaultRiskFree, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name      string
		got, want map[string]float64
	}{{"prior", got.Prior, exp.BL.Prior}, {"views", got.Views, exp.BL.Views}, {"posterior", got.Posterior, exp.BL.Posterior}} {
		if len(c.got) != len(c.want) {
			t.Fatalf("%s: %d vs %d tickers", c.name, len(c.got), len(c.want))
		}
		for tk, w := range c.want {
			if !near(c.got[tk], w+DefaultRiskFree, 1e-9) {
				t.Errorf("%s[%s]: %v vs %v", c.name, tk, c.got[tk], w+DefaultRiskFree)
			}
		}
	}
	sum := 0.0
	for _, w := range got.Weights {
		sum += w
		if w < -1e-9 {
			t.Errorf("peso negativo %v", w)
		}
	}
	if math.Abs(sum-1) > 1e-6 {
		t.Errorf("suman %v", sum)
	}
}

// Propiedad básica de BL: sin views (scores = 50) el tangente recupera el portafolio de mercado.
func TestBLConViewsNeutrasRecuperaElMercado(t *testing.T) {
	rows, _ := loadRun(t)
	exp := loadExpected(t)
	r, err := LoadPriceMatrix(runDir, exp.Tickers)
	if err != nil {
		t.Fatal(err)
	}
	caps, _ := capsScores(rows)
	scores := map[string]*float64{}
	total := 0.0
	for _, tk := range exp.Tickers {
		scores[tk] = fp(50)
		total += *caps[tk]
	}
	got, err := BlackLittermanWeights(r, caps, scores, DefaultRiskFree, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	for tk, w := range got.Weights {
		if math.Abs(w-*caps[tk]/total) > 2e-3 {
			t.Errorf("%s: %v vs mercado %v", tk, w, *caps[tk]/total)
		}
	}
}

func TestSeleccionPorClusterYPortafolioCompleto(t *testing.T) {
	rows, m := loadRun(t)
	caps, scores := capsScores(rows)
	var tickers []string
	for _, r := range rows[:30] {
		tickers = append(tickers, r.Ticker)
	}
	r, err := LoadPriceMatrix(runDir, tickers)
	if err != nil {
		t.Fatal(err)
	}
	_ = m
	b, err := BuildPortfolio(r, caps, scores, DefaultRiskFree, nil, 8, 0.25)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Selection) != 8 || len(b.BL.Weights) != 8 {
		t.Fatalf("selección %d, pesos %d", len(b.Selection), len(b.BL.Weights))
	}
	for i := 1; i < len(b.Selection); i++ {
		a, c := b.Selection[i-1], b.Selection[i]
		if a.Score != nil && c.Score != nil && *a.Score < *c.Score {
			t.Errorf("la selección debe ir por score descendente: %v antes de %v", *a.Score, *c.Score)
		}
		if c.Cluster != i+1 {
			t.Errorf("cluster %d en la posición %d", c.Cluster, i)
		}
	}
	for tk, w := range b.BL.Weights {
		if w > 0.25+1e-6 || w < -1e-9 {
			t.Errorf("%s: peso %v fuera de [0, 0.25]", tk, w)
		}
	}
	if b.MaxWeight != 0.25 {
		t.Errorf("max_weight %v", b.MaxWeight)
	}
	// topes < 1/n se elevan a 1/n (no existe solución que sume 1)
	b2, err := BuildPortfolio(r, caps, scores, DefaultRiskFree, nil, 10, 0.05)
	if err != nil || math.Abs(b2.MaxWeight-0.1) > 1e-12 {
		t.Fatal(b2, err)
	}
	if _, err := BuildPortfolio(r, caps, scores, DefaultRiskFree, nil, 10, 0.01); err == nil {
		t.Fatal("tope fuera de rango debe fallar")
	}
	if _, err := BuildPortfolio(r, caps, scores, DefaultRiskFree, nil, 1, 0.25); err == nil {
		t.Fatal("n_assets fuera de rango debe fallar")
	}
}

func TestLoadPriceMatrixErrores(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadPriceMatrix(dir, []string{"AAA"}); err == nil || !strings.Contains(err.Error(), "al menos 2") {
		t.Fatal(err)
	}
	if _, err := LoadPriceMatrix(dir, []string{"AAA", "BBB"}); err == nil || !strings.Contains(err.Error(), "prices.json") {
		t.Fatal(err)
	}
	dates := make([]string, 60)
	col := make([]float64, 60)
	for i := range dates {
		dates[i] = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format("2006-01-02")
		col[i] = 100 + float64(i)
	}
	write := func(v any) {
		b, _ := json.Marshal(v)
		if err := os.WriteFile(filepath.Join(dir, "prices.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(map[string]any{"dates": dates, "prices": map[string]any{"AAA": col}})
	if _, err := LoadPriceMatrix(dir, []string{"AAA", "ZZZ"}); err == nil || !strings.Contains(err.Error(), "ZZZ") {
		t.Fatal(err)
	}
	write(map[string]any{"dates": dates[:10], "prices": map[string]any{"AAA": col[:10], "BBB": col[:10]}})
	if _, err := LoadPriceMatrix(dir, []string{"AAA", "BBB"}); err == nil || !strings.Contains(err.Error(), "insuficiente") {
		t.Fatal(err)
	}
	write(map[string]any{"dates": dates, "prices": map[string]any{"AAA": col, "BBB": col}})
	r, err := LoadPriceMatrix(dir, []string{"AAA", "BBB"})
	if err != nil || len(r.Cols) != 2 || len(r.Cols[0]) != 59 {
		t.Fatal(r, err)
	}
}

func TestSinExcesoDeRetornoEsErrorDeDatos(t *testing.T) {
	// Original: SLSQP devolvía un portafolio sin sentido. Go: error claro (docs/PARIDAD.md, corrección).
	rows, _ := loadRun(t)
	caps, scores := capsScores(rows)
	exp := loadExpected(t)
	r, err := LoadPriceMatrix(runDir, exp.Tickers)
	if err != nil {
		t.Fatal(err)
	}
	// views muy negativas sobre todos → posterior bajo la tasa libre
	views := map[string]float64{}
	for _, tk := range exp.Tickers {
		views[tk] = -0.5
	}
	if _, err := BlackLittermanWeights(r, caps, scores, 0.5, views, 1); err == nil || !strings.Contains(err.Error(), "tasa libre") {
		t.Fatalf("esperaba error de tasa libre: %v", err)
	}
}

func TestPriceSubset(t *testing.T) {
	v := func(x float64) *float64 { return &x }
	m := PriceMatrix{Dates: []string{"a", "b", "c"}, Prices: map[string][]*float64{"X": {v(1), v(2), v(3)}, "Y": {v(4), nil, v(6)}}}
	got := PriceSubset(m, 2, []string{"Y"})
	if len(got.Dates) != 2 || got.Dates[0] != "b" || len(got.Prices) != 1 || got.Prices["Y"][1] == nil || *got.Prices["Y"][1] != 6 {
		t.Fatalf("%+v", got)
	}
	if all := PriceSubset(m, 0, nil); len(all.Dates) != 3 || len(all.Prices) != 2 {
		t.Fatal("sin filtros devuelve todo")
	}
}

func TestEvalCacheTTL(t *testing.T) {
	c := NewEvalCache()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return now }
	c.Put("AAPL", TickerEvaluation{Ticker: "AAPL"})
	if v, ok := c.Get("AAPL"); !ok || v.Ticker != "AAPL" {
		t.Fatal("debe estar en caché")
	}
	now = now.Add(EvalTTL + time.Second)
	if _, ok := c.Get("AAPL"); ok {
		t.Fatal("vencida")
	}
	for i := 0; i < EvalMax+10; i++ {
		c.Put(string(rune(1000+i)), TickerEvaluation{})
	}
	if len(c.data) > EvalMax {
		t.Fatalf("la caché creció a %d", len(c.data))
	}
}

func TestHistoryEscribeYLee(t *testing.T) {
	dir := t.TempDir()
	runs := filepath.Join(dir, "us_stocks", "runs")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(runDir, "result.json"))
	// dos corridas copiadas del fixture con distinto run_id
	for _, id := range []string{"2026-09-25T21-00-01Z", "2026-09-26T21-00-01Z"} {
		d := filepath.Join(runs, id)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, f := range []string{"result.json", "candidates.json", "prices.json"} {
			src, _ := os.ReadFile(filepath.Join(runDir, f))
			src = []byte(strings.ReplaceAll(string(src), "2026-09-25T21-00-01Z", id))
			if err := os.WriteFile(filepath.Join(d, f), src, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	_ = b
	loc, _ := time.LoadLocation("America/Santiago")
	s := &core.Settings{Env: "test", DataDir: dir, Loc: loc}
	msg, err := Backfill(s, t.Logf)
	if err != nil || msg != "backfill: 2 corridas cargadas en history.db" {
		t.Fatal(msg, err)
	}
	cs, e := Corridas(s)
	if e != nil || len(cs) != 2 || cs[0].TopN != 15 || cs[0].Top1Ticker == nil {
		t.Fatal(cs, e)
	}
	d, e := Diff(s, "", "")
	if e != nil || d.Desde != "2026-09-25T21-00-01Z" || d.TopN == nil || *d.TopN != 15 {
		t.Fatal(d, e)
	}
	for _, f := range d.Filas {
		if f.Estado != "sigue" && f.Estado != "fuera" {
			t.Errorf("corridas idénticas: solo sigue/fuera, hay %s", f.Estado)
		}
	}
	top, e := PermanenciaTop(s, 20)
	if e != nil || len(top) != 15 || top[0].VecesEnTop != 2 || top[0].RachaActual != 2 || top[0].Corridas != 2 || top[0].RankUltima == nil {
		t.Fatal(top, e)
	}
	h, e := HistoriaTicker(s, Top1(t, cs))
	if e != nil || len(h) != 2 || !h[0].InTop || h[0].Rank != 1 {
		t.Fatal(h, e)
	}
	if _, e := HistoriaTicker(s, "ZZZZ"); e == nil || e.Status != 404 {
		t.Fatal("ticker inexistente → 404")
	}
	// idempotente
	if _, err := Backfill(s, t.Logf); err != nil {
		t.Fatal(err)
	}
	if cs2, _ := Corridas(s); len(cs2) != 2 {
		t.Fatal("el backfill duplicó corridas")
	}
}

// Top1 devuelve el ticker #1 de la primera corrida.
func Top1(t *testing.T, cs []UsHistCorrida) string {
	t.Helper()
	return *cs[0].Top1Ticker
}
