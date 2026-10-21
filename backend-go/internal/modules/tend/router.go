package tend

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/api"
)

const tag = "Tendencia"

// Register agrega los endpoints de /api/tendencia:
//   - /latest y /runs/{run_dir}/dashboard: una corrida completa;
func Register(h *api.Host) {
	op := func(id, path, summary string) huma.Operation {
		return huma.Operation{OperationID: id, Method: http.MethodGet, Path: "/api/tendencia" + path, Summary: summary, Tags: []string{tag}}
	}
	const runPat = `^\d{8}_\d{2}$`
	const tickerPat = "^[A-Za-z0-9.\\-^=]{1,15}$"

	api.Public(h, op("td_latest", "/latest", "Última corrida completa"),
		func(context.Context, *struct{}) (*struct{ Body TdDashboard }, error) {
			d, e := Dashboard(h.S, "")
			if e != nil {
				return nil, e
			}
			return &struct{ Body TdDashboard }{*d}, nil
		})
	api.Public(h, op("td_run_dashboard", "/runs/{run_dir}/dashboard", "Una corrida completa"),
		func(_ context.Context, in *struct {
			RunDir string `path:"run_dir" pattern:"^\\d{8}_\\d{2}$" doc:"YYYYMMDD_NN"`
		}) (*struct{ Body TdDashboard }, error) {
			d, e := Dashboard(h.S, in.RunDir)
			if e != nil {
				return nil, e
			}
			return &struct{ Body TdDashboard }{*d}, nil
		})
	api.Public(h, op("td_excluidos", "/excluidos", "Tickers excluidos en el último armado del dataset (con motivo)"),
		func(context.Context, *struct{}) (*struct{ Body []TdExcluido }, error) {
			return &struct{ Body []TdExcluido }{Excluidos(h.S)}, nil
		})
}
