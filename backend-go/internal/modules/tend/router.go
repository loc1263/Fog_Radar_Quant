package tend

import (
	"context"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/api"
)

const tag = "Tendencia"

// Register agrega los endpoints de /api/tendencia:
//   - /latest y /runs/{run_dir}/dashboard: una corrida completa;
//   - /runs/{run_dir}/files[/{nombre}]: PDF y CSV de una corrida;
func Register(h *api.Host) {
	op := func(id, path, summary string) huma.Operation {
		return huma.Operation{OperationID: id, Method: http.MethodGet, Path: "/api/tendencia" + path, Summary: summary, Tags: []string{tag}}
	}
	const runPat = `^\d{8}_\d{2}$`
	const tickerPat = "^[A-Za-z0-9.\\-^=]{1,15}$"

	api.Protected(h, op("td_latest", "/latest", "Última corrida completa"),
		func(context.Context, *struct{}) (*struct{ Body TdDashboard }, error) {
			d, e := Dashboard(h.S, "")
			if e != nil {
				return nil, e
			}
			return &struct{ Body TdDashboard }{*d}, nil
		})
	api.Protected(h, op("td_run_dashboard", "/runs/{run_dir}/dashboard", "Una corrida completa"),
		func(_ context.Context, in *struct {
			RunDir string `path:"run_dir" pattern:"^\\d{8}_\\d{2}$" doc:"YYYYMMDD_NN"`
		}) (*struct{ Body TdDashboard }, error) {
			d, e := Dashboard(h.S, in.RunDir)
			if e != nil {
				return nil, e
			}
			return &struct{ Body TdDashboard }{*d}, nil
		})
	_ = runPat
	api.Protected(h, op("td_run_files", "/runs/{run_dir}/files", "Archivos de una corrida"),
		func(_ context.Context, in *struct {
			RunDir string `path:"run_dir" pattern:"^\\d{8}_\\d{2}$" doc:"YYYYMMDD_NN"`
		}) (*struct{ Body []TdArchivo }, error) {
			d, e := RunDir(h.S, in.RunDir)
			if e != nil {
				return nil, e
			}
			return &struct{ Body []TdArchivo }{Archivos(d)}, nil
		})

	// Descarga de un archivo (PDF, CSV, JSON, TXT): respuesta binaria.
	fop := op("td_run_file", "/runs/{run_dir}/files/{nombre}", "Descarga un archivo de una corrida (PDF, CSV, JSON, TXT)")
	fop.Responses = map[string]*huma.Response{"200": {Description: "OK", Content: map[string]*huma.MediaType{
		"application/pdf": {}, "text/csv": {}}}}
	api.Protected(h, fop, func(_ context.Context, in *struct {
		RunDir string `path:"run_dir" pattern:"^\\d{8}_\\d{2}$" doc:"YYYYMMDD_NN"`
		Nombre string `path:"nombre"`
	}) (*huma.StreamResponse, error) {
		d, e := RunDir(h.S, in.RunDir)
		if e != nil {
			return nil, e
		}
		path, tipo, e := Archivo(d, in.Nombre)
		if e != nil {
			return nil, e
		}
		return &huma.StreamResponse{Body: func(ctx huma.Context) {
			ctx.SetHeader("Content-Type", tipo)
			ctx.SetHeader("Content-Disposition", `attachment; filename="`+in.Nombre+`"`)
			f, err := os.Open(path)
			if err != nil {
				ctx.SetStatus(http.StatusNotFound)
				return
			}
			defer f.Close()
			buf := make([]byte, 64*1024)
			w := ctx.BodyWriter()
			for {
				n, rerr := f.Read(buf)
				if n > 0 {
					if _, werr := w.Write(buf[:n]); werr != nil {
						return
					}
				}
				if rerr != nil {
					return
				}
			}
		}}, nil
	})

	api.Protected(h, op("td_excluidos", "/excluidos", "Tickers excluidos en el último armado del dataset (con motivo)"),
		func(context.Context, *struct{}) (*struct{ Body []TdExcluido }, error) {
			return &struct{ Body []TdExcluido }{Excluidos(h.S)}, nil
		})
}
