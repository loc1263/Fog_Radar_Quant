package us

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// Top1 devuelve el ticker #1 de la primera corrida.
func Top1(t *testing.T, cs []UsHistCorrida) string {
	t.Helper()
	return *cs[0].Top1Ticker
}
