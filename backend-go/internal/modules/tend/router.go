package tend

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/api"
)

const tag = "Tendencia"

// Register agrega los endpoints de /api/tendencia:
//   - /latest: la última corrida completa.
func Register(h *api.Host) {
	op := func(id, path, summary string) huma.Operation {
		return huma.Operation{OperationID: id, Method: http.MethodGet, Path: "/api/tendencia" + path, Summary: summary, Tags: []string{tag}}
	}

	api.Public(h, op("td_latest", "/latest", "Última corrida completa"),
		func(context.Context, *struct{}) (*struct{ Body TdDashboard }, error) {
			d, e := Dashboard(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body TdDashboard }{*d}, nil
		})
}
