package cl

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/api"
)

// Register agrega los endpoints de Chile: /ranking, /indicadores y /history/*.
func Register(h *api.Host) {
	api.Protected(h, huma.Operation{
		OperationID: "cl_ranking", Method: http.MethodGet, Path: "/api/cl/ranking",
		Summary: "Último ranking multi-factor del IPSA", Tags: []string{tag},
	}, func(context.Context, *struct{}) (*struct{ Body RankingResponse }, error) {
		out, e := Ranking(h.S)
		if e != nil {
			return nil, e
		}
		return &struct{ Body RankingResponse }{*out}, nil
	})

	api.Protected(h, huma.Operation{
		OperationID: "cl_indicadores", Method: http.MethodGet, Path: "/api/cl/indicadores",
		Summary: "Indicadores financieros crudos (antes del scoring)", Tags: []string{tag},
	}, func(context.Context, *struct{}) (*struct{ Body IndicadoresResponse }, error) {
		out, e := Indicadores(h.S)
		if e != nil {
			return nil, e
		}
		return &struct{ Body IndicadoresResponse }{*out}, nil
	})

	registerHistory(h)
}
