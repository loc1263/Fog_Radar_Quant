package us

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
