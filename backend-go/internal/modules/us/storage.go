package us

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"fogradar/internal/core"
)

const key = core.ModuleKey("us_stocks")

// NoRuns es el mensaje cuando todavía no hay corridas.
const NoRuns = "Todavía no hay corridas del screener EEUU. Ejecuta: fogradar job us_stocks"

// RunsDir es data/us_stocks/runs.
func RunsDir(s *core.Settings) string { return filepath.Join(s.ModuleDataDir(key), "runs") }

// ListRuns lista las carpetas de corridas, la más reciente primero (el nombre es un timestamp UTC).
func ListRuns(s *core.Settings) []string {
	ents, err := os.ReadDir(RunsDir(s))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() && e.Name()[0] != '.' {
			out = append(out, filepath.Join(RunsDir(s), e.Name()))
		}
	}
	sort.Slice(out, func(i, j int) bool { return filepath.Base(out[i]) > filepath.Base(out[j]) })
	return out
}

// LatestFinishedRun es la corrida más reciente que ya escribió result.json (una corrida en curso crea su
// carpeta antes de terminar: se ignora y se sirve la última terminada).
func LatestFinishedRun(s *core.Settings) (string, *core.APIError) {
	for _, d := range ListRuns(s) {
		if st, err := os.Stat(filepath.Join(d, "result.json")); err == nil && !st.IsDir() {
			return d, nil
		}
	}
	return "", core.NoData(NoRuns)
}

// ReadRunFile lee y decodifica un JSON de la corrida.
func ReadRunFile(runDir, name string, v any) *core.APIError {
	b, err := os.ReadFile(filepath.Join(runDir, name))
	if err != nil {
		return core.NoData(fmt.Sprintf("La corrida %s no tiene %s.", filepath.Base(runDir), name))
	}
	if err := json.Unmarshal(b, v); err != nil {
		return core.Err(503, "invalid_data", fmt.Sprintf("No se pudo leer %s de %s: %v", name, filepath.Base(runDir), err))
	}
	return nil
}

// PriceSubset recorta la matriz de cierres a los últimos days días y/o a tickers.
func PriceSubset(m PriceMatrix, days int, tickers []string) PriceMatrix {
	prices := m.Prices
	if tickers != nil {
		want := map[string]bool{}
		for _, t := range tickers {
			want[t] = true
		}
		prices = map[string][]*float64{}
		for t, v := range m.Prices {
			if want[t] {
				prices[t] = v
			}
		}
	}
	dates := m.Dates
	if days > 0 && days < len(dates) {
		dates = dates[len(dates)-days:]
		cut := map[string][]*float64{}
		for t, v := range prices {
			cut[t] = v[len(v)-days:]
		}
		prices = cut
	}
	if dates == nil {
		dates = []string{}
	}
	if prices == nil {
		prices = map[string][]*float64{}
	}
	return PriceMatrix{Dates: dates, Prices: prices}
}
