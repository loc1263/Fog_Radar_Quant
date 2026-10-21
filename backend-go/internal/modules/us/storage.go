package us

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"fogradar/internal/core"
)

const key = core.ModuleKey("us_stocks")

// NoRuns es el mensaje cuando todavía no hay corridas.
const NoRuns = "Todavía no hay corridas del screener EEUU. Ejecuta: fogradar job us_stocks"

// LatestFinishedRun es la corrida terminada más reciente (una corrida en curso o fallida no tiene run.json:
// se ignora y se sirve la última terminada).
func LatestFinishedRun(s *core.Settings) (string, *core.APIError) {
	if d := core.LatestRun(s, key); d != "" {
		return d, nil
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
