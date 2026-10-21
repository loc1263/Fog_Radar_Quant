package etf

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/api"
)

const tag = "ETFs"

// Register agrega los endpoints de /api/etf. El GET /api/status del proyecto original es GET /api/etf/resumen. Todas las lecturas toman el lock de data/etf/current (503 «busy» si
// el pipeline lo tiene más de 15 s).
func Register(h *api.Host) {
	op := func(id, method, path, summary string) huma.Operation {
		return huma.Operation{OperationID: id, Method: method, Path: "/api/etf" + path, Summary: summary, Tags: []string{tag}}
	}
	tabla := func(id, path, summary, nombre, detalle string) {
		api.Public(h, op(id, http.MethodGet, path, summary), func(context.Context, *struct{}) (*struct{ Body EtfTabla }, error) {
			t, e := Tabla(h.S, nombre, detalle)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfTabla }{*t}, nil
		})
	}


	api.Public(h, op("etf_resumen", http.MethodGet, "/resumen", "Resumen de la corrida vigente + parámetros del modelo"),
		func(context.Context, *struct{}) (*struct{ Body EtfResumen }, error) {
			r, e := Resumen(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfResumen }{*r}, nil
		})
	tabla("etf_top15", "/top15", "Top 15 recomendados (ranking principal)", Top15File, "")
	tabla("etf_top15_simple", "/top15-simple", "Top 15 por rendimiento y Beta (20 % c/u)", Top15SimpleFile, "")
	api.Public(h, op("etf_top15_vanguard", http.MethodGet, "/top15-vanguard", "Top 15 del ranking principal, solo Vanguard"),
		func(context.Context, *struct{}) (*struct{ Body EtfTabla }, error) {
			t, e := Top15Vanguard(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfTabla }{*t}, nil
		})
	tabla("etf_ranking_completo", "/ranking-completo", "Universo depurado completo", RankingFile, "")
	tabla("etf_excluidos", "/excluidos", "Excluidos por distorsión (|z MAD| > umbral)", ExcluidosFile, "")
	api.Public(h, op("etf_diversificacion", http.MethodGet, "/diversificacion", "Categorías dentro del Top 15"),
		func(context.Context, *struct{}) (*struct{ Body EtfDiversificacion }, error) {
			r, e := Diversificacion(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfDiversificacion }{*r}, nil
		})
	api.Public(h, op("etf_rotacion", http.MethodGet, "/rotacion", "Rotación del Top 15 vs. la corrida anterior"),
		func(context.Context, *struct{}) (*struct{ Body EtfRotacion }, error) {
			r, e := RotacionVsAnterior(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfRotacion }{*r}, nil
		})
	tabla("etf_momentum_top15", "/momentum/top15", "Top 15 Momentum / crecimiento", MomentumTop15, "El análisis de Momentum no se corrió en esta corrida.")
	api.Public(h, op("etf_categorias_ranking", http.MethodGet, "/categorias/ranking", "Ranking por categoría"),
		func(context.Context, *struct{}) (*struct{ Body EtfCategorias }, error) {
			r, e := Categorias(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfCategorias }{*r}, nil
		})
	api.Public(h, op("etf_russell3000", http.MethodGet, "/russell3000", "Top 15 Cap. (momentum factor tipo VFMO)"),
		func(context.Context, *struct{}) (*struct{ Body EtfCap }, error) {
			r, e := Top15Cap(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body EtfCap }{*r}, nil
		})
}
