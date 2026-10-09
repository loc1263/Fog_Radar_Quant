package etf

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/api"
	"fogradar/internal/core"
	"fogradar/internal/jobs"
)

const tag = "ETFs"

// Register agrega los endpoints de /api/etf. Mismos endpoints que ETF_Choice/WEB/backend/main.py bajo el prefijo del módulo;
// el GET /api/status del original es GET /api/etf/resumen. Todas las lecturas toman el lock de data/etf/current (503 «busy» si
// el pipeline lo tiene más de 15 s). POST /run re-analiza el CSV vigente (sin Yahoo) y queda en el registro de jobs.
//
// GET /pdf (informe ejecutivo) se genera con internal/pdf (Carlito) y se cachea por la firma de los CSV.
func Register(h *api.Host, env func() *jobs.Env) {
	op := func(id, method, path, summary string) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: "/api/etf" + path, Summary: summary, Tags: []string{tag}}
	}
	tabla := func(id, path, summary, nombre, detalle string) {
		api.Protected(h, op(id, http.MethodGet, path, summary), func(context.Context, *struct{}) (*struct{ Body EtfTabla }, error) {
			t, e := Tabla(h.S, nombre, detalle)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfTabla }{*t}, nil
		})
	}

	registerPDF(h)

	api.Protected(h, op("etf_resumen", http.MethodGet, "/resumen", "Resumen de la corrida vigente + parámetros del modelo"),
		func(context.Context, *struct{}) (*struct{ Body EtfResumen }, error) {
			r, e := Resumen(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfResumen }{*r}, nil
		})
	tabla("etf_top15", "/top15", "Top 15 recomendados (ranking principal)", Top15File, "")
	tabla("etf_top15_simple", "/top15-simple", "Top 15 por rendimiento y Beta (20 % c/u)", Top15SimpleFile, "")
	api.Protected(h, op("etf_top15_vanguard", http.MethodGet, "/top15-vanguard", "Top 15 del ranking principal, solo Vanguard"),
		func(context.Context, *struct{}) (*struct{ Body EtfTabla }, error) {
			t, e := Top15Vanguard(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfTabla }{*t}, nil
		})
	tabla("etf_ranking_completo", "/ranking-completo", "Universo depurado completo", RankingFile, "")
	tabla("etf_excluidos", "/excluidos", "Excluidos por distorsión (|z MAD| > umbral)", ExcluidosFile, "")
	api.Protected(h, op("etf_diversificacion", http.MethodGet, "/diversificacion", "Categorías dentro del Top 15"),
		func(context.Context, *struct{}) (*struct{ Body EtfDiversificacion }, error) {
			r, e := Diversificacion(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfDiversificacion }{*r}, nil
		})
	api.Protected(h, op("etf_rotacion", http.MethodGet, "/rotacion", "Rotación del Top 15 vs. la corrida anterior"),
		func(context.Context, *struct{}) (*struct{ Body EtfRotacion }, error) {
			r, e := RotacionVsAnterior(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfRotacion }{*r}, nil
		})
	tabla("etf_momentum_top15", "/momentum/top15", "Top 15 Momentum / crecimiento", MomentumTop15, "El análisis de Momentum no se corrió en esta corrida.")
	api.Protected(h, op("etf_categorias_ranking", http.MethodGet, "/categorias/ranking", "Ranking por categoría"),
		func(context.Context, *struct{}) (*struct{ Body EtfCategorias }, error) {
			r, e := Categorias(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfCategorias }{*r}, nil
		})
	api.Protected(h, op("etf_russell3000", http.MethodGet, "/russell3000", "Top 15 Cap. (momentum factor tipo VFMO)"),
		func(context.Context, *struct{}) (*struct{ Body EtfCap }, error) {
			r, e := Top15Cap(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfCap }{*r}, nil
		})
	api.Protected(h, op("etf_historial", http.MethodGet, "/historial", "Historial de corridas"),
		func(context.Context, *struct{}) (*struct{ Body EtfHistorial }, error) {
			r, e := HistorialCorridas(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfHistorial }{*r}, nil
		})

	// POST /run: re-análisis (sin Yahoo). Mismo lock global que las corridas batch.
	api.ProtectedWith(h, op("etf_run", http.MethodPost, "/run", "Re-analiza el CSV vigente (no descarga de Yahoo)"),
		func(ctx context.Context, _ *struct{}) (*struct{ Body EtfRunResult }, error) {
			reg := h.Jobs
			if reg.IsRunning("etf") {
				return nil, core.Err(http.StatusConflict, "run_in_progress", "Ya hay una corrida de ETFs en curso.")
			}
			unlock, err := core.LockPipelines(h.S, 0)
			if errors.Is(err, core.ErrOcupado) {
				return nil, core.Err(http.StatusConflict, "run_in_progress", "Ya hay una corrida de pipelines en curso.")
			}
			if err != nil {
				return nil, core.Err(http.StatusInternalServerError, "pipeline_error", "Error procesando el análisis; revisa el log del servidor.")
			}
			defer unlock()
			e := env()
			rec, err := reg.Track("etf", "api", func(rec *core.JobRecord) error {
				msg, err := Run(ctx, e, map[string]any{"solo_analisis": true})
				if msg != "" {
					rec.Message = &msg
				}
				return err
			})
			if err != nil {
				var ae *core.APIError
				if errors.As(err, &ae) {
					return nil, ae
				}
				h.Log.Error("error re-analizando ETFs vía POST /run", "error", err)
				return nil, core.Err(http.StatusInternalServerError, "pipeline_error", "Error procesando el análisis; revisa el log del servidor.")
			}
			return &struct{ Body EtfRunResult }{EtfRunResult{Job: *rec}}, nil
		}, h.Extra("3/minute", "etf-run"))

	// --- Historial (history.db): no toman el lock de current ---
	api.Protected(h, op("etf_history_runs", http.MethodGet, "/history/runs", "Corridas guardadas en el historial"),
		func(context.Context, *struct{}) (*struct{ Body []EtfHistCorrida }, error) {
			r, e := Corridas(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body []EtfHistCorrida }{r}, nil
		})
	api.Protected(h, op("etf_history_ticker", http.MethodGet, "/history/ticker/{ticker}", "Score, puesto y métricas de un ETF en cada corrida"),
		func(_ context.Context, in *struct {
			Ticker string `path:"ticker" pattern:"^[A-Za-z0-9.\\-]{1,12}$"`
		}) (*struct{ Body []EtfHistTickerPunto }, error) {
			r, e := HistoriaTicker(h.S, strings.ToUpper(in.Ticker))
			if e != nil {
				return nil, e
			}
			return &struct{ Body []EtfHistTickerPunto }{r}, nil
		})
	api.Protected(h, op("etf_history_diff", http.MethodGet, "/history/diff", "Rotación del Top 15 entre dos corridas"),
		func(_ context.Context, in *struct {
			Desde string `query:"desde" pattern:"^\\d{8}(_\\d+)?$" doc:"yyyymmdd[_n]"`
			Hasta string `query:"hasta" pattern:"^\\d{8}(_\\d+)?$" doc:"yyyymmdd[_n]"`
		}) (*struct{ Body core.HistDiff }, error) {
			r, e := Diff(h.S, in.Desde, in.Hasta)
			if e != nil {
				return nil, e
			}
			return &struct{ Body core.HistDiff }{*r}, nil
		})
	api.Protected(h, op("etf_history_top", http.MethodGet, "/history/top", "Permanencia en el Top 15 (últimas N)"),
		func(_ context.Context, in *struct {
			Ultimas int `query:"ultimas" default:"12" minimum:"2" maximum:"260"`
		}) (*struct{ Body []EtfHistPermanencia }, error) {
			r, e := PermanenciaTop(h.S, in.Ultimas)
			if e != nil {
				return nil, e
			}
			return &struct{ Body []EtfHistPermanencia }{r}, nil
		})
	api.Protected(h, op("etf_history_categorias", http.MethodGet, "/history/categorias", "Score promedio y presencia en el Top 15 por categoría, en el tiempo"),
		func(context.Context, *struct{}) (*struct{ Body []EtfHistCategoria }, error) {
			r, e := HistoriaCategorias(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body []EtfHistCategoria }{r}, nil
		})
}
