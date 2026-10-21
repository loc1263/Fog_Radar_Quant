package us

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/api"
	"fogradar/internal/core"
)

const tag = "Acciones EEUU"

var tickerRe = regexp.MustCompile(TickerPattern)

// Register agrega los endpoints de /api/us: corridas y portafolio precalculado.
func Register(h *api.Host) {
	op := func(id, method, path, summary string) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: "/api/us" + path, Summary: summary, Tags: []string{tag}}
	}

	// --- Corridas ---
	api.Protected(h, op("us_latest_ranking", http.MethodGet, "/rankings/latest", "Top 15 por momentum (última corrida)"),
		func(context.Context, *struct{}) (*struct{ Body RunResponse }, error) {
			var r RunResponse
			if e := readLatest(h.S, "result.json", &r); e != nil {
				return nil, e
			}
			return &struct{ Body RunResponse }{fixRows(r)}, nil
		})
	api.Protected(h, op("us_latest_candidates", http.MethodGet, "/rankings/latest/candidates", "Todos los candidatos evaluados (última corrida)"),
		func(context.Context, *struct{}) (*struct{ Body RunResponse }, error) {
			var r RunResponse
			if e := readLatest(h.S, "candidates.json", &r); e != nil {
				return nil, e
			}
			return &struct{ Body RunResponse }{fixRows(r)}, nil
		})
	api.Protected(h, op("us_latest_prices", http.MethodGet, "/rankings/latest/prices", "Matriz de cierres (última corrida)"),
		func(_ context.Context, in *struct {
			Days    int      `query:"days" minimum:"2" maximum:"400" doc:"Solo los últimos N días."`
			Tickers []string `query:"tickers,explode" maxItems:"150"`
		}) (*struct{ Body PriceMatrix }, error) {
			var m PriceMatrix
			if e := readLatest(h.S, "prices.json", &m); e != nil {
				return nil, e
			}
			var wanted []string
			if len(in.Tickers) > 0 {
				for _, t := range in.Tickers {
					wanted = append(wanted, strings.ToUpper(strings.TrimSpace(t)))
				}
			}
			return &struct{ Body PriceMatrix }{PriceSubset(m, in.Days, wanted)}, nil
		})
	api.Protected(h, op("us_list_runs", http.MethodGet, "/rankings/runs", "Corridas disponibles (más reciente primero)"),
		func(context.Context, *struct{}) (*struct{ Body RunsList }, error) {
			names := []string{}
			for _, d := range ListRuns(h.S) {
				names = append(names, filepath.Base(d))
			}
			return &struct{ Body RunsList }{RunsList{Runs: names}}, nil
		})

	// --- Portafolio ---
	api.Protected(h, op("us_portfolio_limits", http.MethodGet, "/portfolio/limits", "Límites del modelo de portafolio"),
		func(context.Context, *struct{}) (*struct{ Body PortfolioLimits }, error) {
			return &struct{ Body PortfolioLimits }{PortfolioLimits{MinTickers: MinTickers, MinAssets: MinNAssets, MaxAssets: MaxNAssets,
				DefaultAssets: DefaultNAssets, DefaultMaxWeight: DefaultMaxWeight, DefaultRiskFreeRate: DefaultRiskFree}}, nil
		})
	api.Protected(h, op("us_portfolio_precalc", http.MethodGet, "/portfolio/precalculado", "Portafolios precalculados con los parámetros por defecto (universo completo, Top 30, Top 15)"),
		func(context.Context, *struct{}) (*struct{ Body PortfolioPrecalcResponse }, error) {
			res, e := PortfolioPrecalculado(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body PortfolioPrecalcResponse }{*res}, nil
		})
}

// Optimize arma el portafolio del modelo único sobre la corrida terminada más reciente.
func Optimize(s *core.Settings, req PortfolioRequest) (*PortfolioResult, *core.APIError) {
	runDir, e := LatestFinishedRun(s)
	if e != nil {
		return nil, e
	}
	set := map[string]bool{}
	for i, t := range req.Tickers {
		t = strings.ToUpper(strings.TrimSpace(t))
		if !tickerRe.MatchString(t) {
			return nil, core.Err(http.StatusUnprocessableEntity, "validation_error", fmt.Sprintf("body.tickers.%d: Ticker inválido: %q", i, t))
		}
		set[t] = true
	}
	tickers := make([]string, 0, len(set))
	for t := range set {
		tickers = append(tickers, t)
	}
	sort.Strings(tickers)

	fail := func(err error) *core.APIError {
		var pe *PortfolioDataError
		if errors.As(err, &pe) {
			return core.Err(http.StatusBadRequest, "portfolio_data", pe.Msg)
		}
		var ae *core.APIError
		if errors.As(err, &ae) {
			return ae
		}
		return core.Err(http.StatusInternalServerError, "internal_error", err.Error())
	}
	r, err := LoadPriceMatrix(runDir, tickers)
	if err != nil {
		return nil, fail(err)
	}
	caps, scores, e := MarketCapsAndScores(runDir, tickers)
	if e != nil {
		return nil, e
	}
	rf := DefaultRiskFree
	if req.RiskFreeRate != nil {
		rf = *req.RiskFreeRate
	}
	views := map[string]float64{}
	for k, v := range req.Views {
		views[strings.ToUpper(strings.TrimSpace(k))] = v
	}
	nAssets, maxW := req.NAssets, req.MaxWeight
	if nAssets == 0 {
		nAssets = DefaultNAssets
	}
	if maxW == 0 {
		maxW = DefaultMaxWeight
	}
	b, err := BuildPortfolio(r, caps, scores, rf, views, nAssets, maxW)
	if err != nil {
		return nil, fail(err)
	}
	res := &PortfolioResult{RunID: filepath.Base(runDir), Universe: tickers, Weights: b.BL.Weights, MaxWeight: b.MaxWeight,
		Metrics: b.Metrics, PriorReturns: b.BL.Prior, Views: b.BL.Views, PosteriorReturns: b.BL.Posterior}
	for _, p := range b.Selection {
		res.Selection = append(res.Selection, PortfolioPick(p))
	}
	return res, nil
}

func readLatest(s *core.Settings, name string, v any) *core.APIError {
	dir, e := LatestFinishedRun(s)
	if e != nil {
		return e
	}
	return ReadRunFile(dir, name, v)
}

// fixRows garantiza «rows: []» en vez de null.
func fixRows(r RunResponse) RunResponse {
	if r.Rows == nil {
		r.Rows = []CandidateRow{}
	}
	return r
}
