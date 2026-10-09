// Package cl es el módulo Chile (IPSA). En la Fase 1 solo expone el historial de
// corridas (/api/cl/history/*), de solo lectura sobre data/cl_stocks/history.db.
package cl

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/api"
	"fogradar/internal/core"
)

const (
	key = core.ModuleKey("cl_stocks")
	tag = "Chile (IPSA)"
)

// MetricasNumericas son las claves de columnas.py cuya unidad no es «texto» (58).
var MetricasNumericas = func() map[string]bool {
	m := map[string]bool{}
	for _, k := range strings.Split("tasa_cambio,market_cap,enterprise_value,trailing_pe,forward_pe,peg,"+
		"price_sales,price_book,ev_revenue,ev_ebitda,profit_margin,operating_margin,roa,roe,revenue_growth_q,"+
		"earnings_growth_q,revenue,revenue_per_share,gross_profit,ebitda,net_income,eps,operating_cash_flow,"+
		"levered_fcf,total_cash,cash_per_share,total_debt,debt_equity,current_ratio,book_value_per_share,beta,"+
		"change_52w,sp500_change_52w,high_52w,low_52w,sma_50,sma_200,ret_5d,ret_1m,ret_3m,ret_6m,ret_ytd,ret_1a,"+
		"avg_vol_3m,avg_vol_10d,shares_outstanding,implied_shares,float_shares,held_insiders,held_institutions,"+
		"dividend_rate,dividend_yield,trailing_dividend_rate,trailing_dividend_yield,dividend_yield_5y,"+
		"payout_ratio,ex_dividend_date,last_split_date", ",") {
		m[k] = true
	}
	return m
}()

// ClHistCorrida es una corrida guardada en el historial.
type ClHistCorrida struct {
	Ts         string     `json:"ts" doc:"Timestamp de la corrida (YYYYMMDD_HHMMSS), su identificador."`
	RunAt      core.Fecha `json:"run_at"`
	Archivo    string     `json:"archivo"`
	NTickers   int64      `json:"n_tickers"`
	Top1Ticker *string    `json:"top1_ticker" nullable:"true"`
	Top1Score  *float64   `json:"top1_score" nullable:"true"`
}

// PilarScores es el z-score promedio de cada pilar (null si no hay métricas en ese pilar).
type PilarScores struct {
	Valor           *float64 `json:"valor" nullable:"true"`
	Calidad         *float64 `json:"calidad" nullable:"true"`
	Crecimiento     *float64 `json:"crecimiento" nullable:"true"`
	SaludFinanciera *float64 `json:"salud_financiera" nullable:"true"`
	Dividendo       *float64 `json:"dividendo" nullable:"true"`
	Momentum        *float64 `json:"momentum" nullable:"true"`
}

// ClHistTickerPunto es la posición de una acción en una corrida.
type ClHistTickerPunto struct {
	Ts                 string      `json:"ts"`
	RunAt              core.Fecha  `json:"run_at"`
	NTickers           int64       `json:"n_tickers"`
	Rank               *int64      `json:"rank" nullable:"true"`
	Nombre             *string     `json:"nombre" nullable:"true"`
	ScoreCompuesto     *float64    `json:"score_compuesto" nullable:"true"`
	Pilares            PilarScores `json:"pilares"`
	MetricasDisponible *int64      `json:"metricas_disponibles" nullable:"true"`
}

// ClHistIndicadorPunto es el valor de un indicador crudo en una corrida.
type ClHistIndicadorPunto struct {
	Ts    string     `json:"ts"`
	RunAt core.Fecha `json:"run_at"`
	Valor *float64   `json:"valor" nullable:"true"`
}

type store struct{ h *api.Host }

func (s store) path() string {
	return filepath.Join(s.h.S.ModuleDataDir(key), core.HistDBName)
}

func (s store) runAt(f core.Fila, col string) (core.Fecha, *core.APIError) {
	raw := f.Str(col)
	if raw == nil {
		return core.Fecha{}, core.Err(http.StatusServiceUnavailable, "invalid_data", "history.db: run_at vacío")
	}
	t, err := core.ParseISO(*raw, s.h.S.Loc)
	if err != nil {
		return core.Fecha{}, core.Err(http.StatusServiceUnavailable, "invalid_data", "history.db: "+err.Error())
	}
	return core.Fecha{Time: t}, nil
}

func (s store) corridas() ([]ClHistCorrida, *core.APIError) {
	rows, e := core.Filas(s.path(), `
		SELECT r.ts, r.run_at, r.archivo, r.n_tickers,
		       k.ticker AS top1_ticker, k.score_compuesto AS top1_score
		FROM runs r
		LEFT JOIN ranking k ON k.run_pk = r.id AND k.rank = 1
		ORDER BY r.ts ASC`)
	if e != nil {
		return nil, e
	}
	out := make([]ClHistCorrida, 0, len(rows))
	for _, f := range rows {
		at, e := s.runAt(f, "run_at")
		if e != nil {
			return nil, e
		}
		out = append(out, ClHistCorrida{
			Ts: deref(f.Str("ts")), RunAt: at, Archivo: deref(f.Str("archivo")),
			NTickers: derefI(f.Int("n_tickers")), Top1Ticker: f.Str("top1_ticker"), Top1Score: f.Float("top1_score"),
		})
	}
	return out, nil
}

func (s store) historiaTicker(ticker string) ([]ClHistTickerPunto, *core.APIError) {
	rows, e := core.Filas(s.path(), `
		SELECT r.ts, r.run_at, r.n_tickers, k.rank, k.nombre, k.score_compuesto,
		       k.score_valor, k.score_calidad, k.score_crecimiento, k.score_salud_financiera,
		       k.score_dividendo, k.score_momentum, k.metricas_disponibles
		FROM ranking k
		JOIN runs r ON r.id = k.run_pk
		WHERE k.ticker = ?
		ORDER BY r.ts ASC`, ticker)
	if e != nil {
		return nil, e
	}
	if len(rows) == 0 {
		return nil, core.Err(http.StatusNotFound, "not_found",
			fmt.Sprintf("'%s' no aparece en el historial del IPSA.", ticker))
	}
	out := make([]ClHistTickerPunto, 0, len(rows))
	for _, f := range rows {
		at, e := s.runAt(f, "run_at")
		if e != nil {
			return nil, e
		}
		out = append(out, ClHistTickerPunto{
			Ts: deref(f.Str("ts")), RunAt: at, NTickers: derefI(f.Int("n_tickers")),
			Rank: f.Int("rank"), Nombre: f.Str("nombre"), ScoreCompuesto: f.Float("score_compuesto"),
			Pilares: PilarScores{
				Valor: f.Float("score_valor"), Calidad: f.Float("score_calidad"),
				Crecimiento: f.Float("score_crecimiento"), SaludFinanciera: f.Float("score_salud_financiera"),
				Dividendo: f.Float("score_dividendo"), Momentum: f.Float("score_momentum"),
			},
			MetricasDisponible: f.Int("metricas_disponibles"),
		})
	}
	return out, nil
}

func (s store) historiaIndicador(ticker, metrica string) ([]ClHistIndicadorPunto, *core.APIError) {
	rows, e := core.Filas(s.path(), `
		SELECT r.ts, r.run_at, i.valor
		FROM indicadores i
		JOIN runs r ON r.id = i.run_pk
		WHERE i.ticker = ? AND i.metrica = ?
		ORDER BY r.ts ASC`, ticker, metrica)
	if e != nil {
		return nil, e
	}
	out := make([]ClHistIndicadorPunto, 0, len(rows))
	for _, f := range rows {
		at, e := s.runAt(f, "run_at")
		if e != nil {
			return nil, e
		}
		out = append(out, ClHistIndicadorPunto{Ts: deref(f.Str("ts")), RunAt: at, Valor: f.Float("valor")})
	}
	return out, nil
}

func (s store) rankingDe(ts string) ([]core.Posicion, *core.APIError) {
	rows, e := core.Filas(s.path(), `
		SELECT k.ticker, k.nombre, k.rank, k.score_compuesto AS score
		FROM ranking k JOIN runs r ON r.id = k.run_pk
		WHERE r.ts = ?`, ts)
	if e != nil {
		return nil, e
	}
	out := make([]core.Posicion, 0, len(rows))
	for _, f := range rows {
		out = append(out, core.Posicion{Ticker: deref(f.Str("ticker")), Nombre: f.Str("nombre"),
			Rank: f.Int("rank"), Score: f.Float("score")})
	}
	return out, nil
}

// diff compara dos corridas (por defecto, las dos últimas).
func (s store) diff(desde, hasta string) (*core.HistDiff, *core.APIError) {
	cs, e := s.corridas()
	if e != nil {
		return nil, e
	}
	tss := make([]string, len(cs))
	for i, c := range cs {
		tss[i] = c.Ts
	}
	switch {
	case desde == "" && hasta == "":
		if len(tss) < 2 {
			return nil, core.Err(http.StatusNotFound, "not_found", "Hacen falta al menos dos corridas en el historial.")
		}
		desde, hasta = tss[len(tss)-2], tss[len(tss)-1]
	case desde == "" || hasta == "":
		return nil, core.Err(http.StatusUnprocessableEntity, "validation_error",
			"Indica 'desde' y 'hasta', o ninguno de los dos.")
	}
	for _, ts := range []string{desde, hasta} {
		if i := sort.SearchStrings(tss, ts); i >= len(tss) || tss[i] != ts {
			return nil, core.Err(http.StatusNotFound, "not_found", fmt.Sprintf("No existe la corrida '%s' en el historial.", ts))
		}
	}
	a, e := s.rankingDe(desde)
	if e != nil {
		return nil, e
	}
	b, e := s.rankingDe(hasta)
	if e != nil {
		return nil, e
	}
	return &core.HistDiff{Desde: desde, Hasta: hasta, Filas: core.DiffCorridas(a, b, nil)}, nil
}

type tickerIn struct {
	Ticker string `path:"ticker" pattern:"^[A-Za-z0-9.\\-]{1,15}$"`
}

// registerHistory agrega los endpoints de historial de Chile.
func registerHistory(h *api.Host) {
	s := store{h}

	api.Protected(h, huma.Operation{
		OperationID: "cl_history_runs", Method: http.MethodGet, Path: "/api/cl/history/runs",
		Summary: "Corridas guardadas en el historial", Tags: []string{tag},
	}, func(context.Context, *struct{}) (*struct{ Body []ClHistCorrida }, error) {
		out, e := s.corridas()
		if e != nil {
			return nil, e
		}
		return &struct{ Body []ClHistCorrida }{out}, nil
	})

	api.Protected(h, huma.Operation{
		OperationID: "cl_history_ticker", Method: http.MethodGet, Path: "/api/cl/history/ticker/{ticker}",
		Summary: "Evolución de una acción", Tags: []string{tag},
	}, func(_ context.Context, in *tickerIn) (*struct{ Body []ClHistTickerPunto }, error) {
		out, e := s.historiaTicker(strings.ToUpper(in.Ticker))
		if e != nil {
			return nil, e
		}
		return &struct{ Body []ClHistTickerPunto }{out}, nil
	})

	api.Protected(h, huma.Operation{
		OperationID: "cl_history_indicador", Method: http.MethodGet,
		Path:    "/api/cl/history/ticker/{ticker}/indicador/{metrica}",
		Summary: "Evolución de un indicador crudo de una acción", Tags: []string{tag},
	}, func(_ context.Context, in *struct {
		Ticker  string `path:"ticker" pattern:"^[A-Za-z0-9.\\-]{1,15}$"`
		Metrica string `path:"metrica"`
	}) (*struct{ Body []ClHistIndicadorPunto }, error) {
		if !MetricasNumericas[in.Metrica] {
			return nil, core.Err(http.StatusNotFound, "not_found", fmt.Sprintf("Métrica desconocida: '%s'.", in.Metrica))
		}
		out, e := s.historiaIndicador(strings.ToUpper(in.Ticker), in.Metrica)
		if e != nil {
			return nil, e
		}
		return &struct{ Body []ClHistIndicadorPunto }{out}, nil
	})

	api.Protected(h, huma.Operation{
		OperationID: "cl_history_diff", Method: http.MethodGet, Path: "/api/cl/history/diff",
		Summary: "Cambios de posición entre dos corridas", Tags: []string{tag},
	}, func(_ context.Context, in *struct {
		Desde string `query:"desde" pattern:"^\\d{8}_\\d{6}$" doc:"YYYYMMDD_HHMMSS"`
		Hasta string `query:"hasta" pattern:"^\\d{8}_\\d{6}$" doc:"YYYYMMDD_HHMMSS"`
	}) (*struct{ Body core.HistDiff }, error) {
		out, e := s.diff(in.Desde, in.Hasta)
		if e != nil {
			return nil, e
		}
		return &struct{ Body core.HistDiff }{*out}, nil
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefI(n *int64) int64 {
	if n == nil {
		return 0
	}
	return *n
}
