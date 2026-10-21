package us

import (
	"fmt"
	"sort"

	"fogradar/internal/core"
)

// SparkDays son los días de cierres que usan las sparklines de tendencia (~3 meses hábiles, como el original).
const SparkDays = 60

// PrecalcTops son los universos «Top N por score» que se precalculan además de «todos los candidatos»
// (los mismos atajos que ofrecía la pantalla de portafolio interactiva).
var PrecalcTops = []int{30, 15}

// PortfolioPrecalc es un portafolio calculado con los parámetros por defecto sobre un universo fijo.
type PortfolioPrecalc struct {
	Clave     string          `json:"clave" doc:"todos | top30 | top15 …"`
	Titulo    string          `json:"titulo"`
	Resultado PortfolioResult `json:"resultado"`
}

// PortfolioPrecalcResponse es la respuesta de /portfolio/precalculado: el sitio estático no puede
// optimizar a pedido, así que muestra estos portafolios.
type PortfolioPrecalcResponse struct {
	RunID       string             `json:"run_id"`
	NAssets     int                `json:"n_assets"`
	MaxWeight   float64            `json:"max_weight"`
	RiskFree    float64            `json:"risk_free_rate"`
	Portafolios []PortfolioPrecalc `json:"portafolios"`
}

// PortfolioPrecalculado optimiza, con los valores por defecto del modelo, el universo completo y los
// Top N por score de la corrida terminada más reciente.
func PortfolioPrecalculado(s *core.Settings) (*PortfolioPrecalcResponse, *core.APIError) {
	var r RunResponse
	if e := readLatest(s, "candidates.json", &r); e != nil {
		return nil, e
	}
	filas := r.Rows
	sort.SliceStable(filas, func(i, j int) bool { return scoreOrden(filas[i]) > scoreOrden(filas[j]) })

	universos := []struct {
		clave, titulo string
		n             int
	}{{"todos", fmt.Sprintf("Todos los candidatos (%d)", len(filas)), len(filas)}}
	for _, n := range PrecalcTops {
		if n < len(filas) {
			universos = append(universos, struct {
				clave, titulo string
				n             int
			}{fmt.Sprintf("top%d", n), fmt.Sprintf("Top %d por score", n), n})
		}
	}

	out := &PortfolioPrecalcResponse{NAssets: DefaultNAssets, MaxWeight: DefaultMaxWeight, RiskFree: DefaultRiskFree,
		Portafolios: []PortfolioPrecalc{}}
	for _, u := range universos {
		tickers := make([]string, 0, u.n)
		for _, f := range filas[:u.n] {
			tickers = append(tickers, f.Ticker)
		}
		res, e := Optimize(s, PortfolioRequest{Tickers: tickers})
		if e != nil {
			return nil, e
		}
		out.RunID = res.RunID
		out.Portafolios = append(out.Portafolios, PortfolioPrecalc{Clave: u.clave, Titulo: u.titulo, Resultado: *res})
	}
	return out, nil
}

func scoreOrden(f CandidateRow) float64 {
	if f.Score.Overall == nil {
		return -1
	}
	return *f.Score.Overall
}
