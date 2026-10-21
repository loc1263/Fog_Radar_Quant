package us

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fogradar/internal/core"
)

func escribirJSON(t *testing.T, dir, nombre string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, nombre), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPortfolioPrecalculado(t *testing.T) {
	s := &core.Settings{DataDir: t.TempDir(), Loc: time.UTC}
	dir, err := core.NewRun(s, key, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := core.FinishRun(dir, key, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}

	tickers := []string{"AAA", "BBB", "CCC", "DDD", "EEE"}
	rng := rand.New(rand.NewSource(1))
	const dias = 300
	fechas := make([]string, dias)
	precios := map[string][]*float64{}
	var filas []CandidateRow
	for i, tk := range tickers {
		p := 100.0
		serie := make([]*float64, dias)
		for d := range serie {
			p *= 1 + 0.0005*float64(i+1) + 0.01*rng.NormFloat64()
			v := p
			serie[d] = &v
		}
		precios[tk] = serie
		cap, sc := 1e9*float64(i+1), float64(10*(i+1))
		filas = append(filas, CandidateRow{Ticker: tk, MarketCap: &cap, Score: Score{Overall: &sc}})
	}
	for d := range fechas {
		fechas[d] = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, d).Format("2006-01-02")
	}
	escribirJSON(t, dir, "candidates.json", RunResponse{Rows: filas})
	escribirJSON(t, dir, "prices.json", PriceMatrix{Dates: fechas, Prices: precios})

	r, e := PortfolioPrecalculado(s)
	if e != nil {
		t.Fatal(e)
	}
	if r.RunID != "20261001_000000" || r.NAssets != DefaultNAssets {
		t.Errorf("respuesta: %+v", r)
	}
	// 5 candidatos: ningún Top N (15, 30) es más chico que el universo, solo «todos».
	if len(r.Portafolios) != 1 || r.Portafolios[0].Clave != "todos" {
		t.Fatalf("portafolios: %+v", r.Portafolios)
	}
	res := r.Portafolios[0].Resultado
	if len(res.Universe) != 5 {
		t.Errorf("universo: %v", res.Universe)
	}
	suma := 0.0
	for _, w := range res.Weights {
		suma += w
	}
	if math.Abs(suma-1) > 1e-6 {
		t.Errorf("los pesos suman %v", suma)
	}
}
