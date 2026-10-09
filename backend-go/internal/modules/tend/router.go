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
//   - /analyses, /runs, /runs/{run_dir}, /history/*: sobre history.db (solo lectura).
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
	api.Protected(h, op("td_analyses", "/analyses", "Análisis vistos en el historial"),
		func(context.Context, *struct{}) (*struct{ Body []TdAnalisisDisponible }, error) {
			r, e := HistAnalyses(h.S)
			if e != nil {
				return nil, e
			}
			return &struct{ Body []TdAnalisisDisponible }{r}, nil
		})

	api.Protected(h, op("td_runs", "/runs", "Performance por corrida (ascendente)"),
		func(_ context.Context, in *struct {
			Analysis string `query:"analysis" pattern:"^(_[a-z]+)?$" doc:"'' = Asia + EEUU (principal), '_us' = solo EEUU"`
			From     string `query:"from" pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
			To       string `query:"to" pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
			Limit    int    `query:"limit" default:"500" minimum:"1" maximum:"5000"`
		}) (*struct{ Body []TdRunPerformance }, error) {
			r, e := Runs(h.S, in.Analysis, in.From, in.To, in.Limit)
			if e != nil {
				return nil, e
			}
			return &struct{ Body []TdRunPerformance }{r}, nil
		})
	api.Protected(h, op("td_run_detalle", "/runs/{run_dir}", "Corrida reconstruida desde history.db"),
		func(_ context.Context, in *struct {
			RunDir string `path:"run_dir" pattern:"^\\d{8}_\\d{2}$" doc:"YYYYMMDD_NN"`
		}) (*struct{ Body TdRunDetalle }, error) {
			r, e := RunDetalle(h.S, in.RunDir)
			if e != nil {
				return nil, e
			}
			return &struct{ Body TdRunDetalle }{*r}, nil
		})
	api.Protected(h, op("td_history_performance", "/history/performance", "Serie de performance"),
		func(_ context.Context, in *struct {
			Analysis string `query:"analysis" pattern:"^(_[a-z]+)?$" doc:"'' = Asia + EEUU (principal), '_us' = solo EEUU"`
			From     string `query:"from" pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
			To       string `query:"to" pattern:"^\\d{4}-\\d{2}-\\d{2}$"`
		}) (*struct{ Body []TdRunPerformance }, error) {
			r, e := Runs(h.S, in.Analysis, in.From, in.To, 5000)
			if e != nil {
				return nil, e
			}
			return &struct{ Body []TdRunPerformance }{r}, nil
		})
	api.Protected(h, op("td_history_tickers", "/history/tickers", "Tickers seleccionados alguna vez"),
		func(_ context.Context, in *struct {
			Analysis string `query:"analysis" pattern:"^(_[a-z]+)?$" doc:"'' = Asia + EEUU (principal), '_us' = solo EEUU"`
		}) (*struct{ Body []TdTickerNombre }, error) {
			r, e := Tickers(h.S, in.Analysis)
			if e != nil {
				return nil, e
			}
			return &struct{ Body []TdTickerNombre }{r}, nil
		})
	_ = tickerPat
	api.Protected(h, op("td_history_ticker", "/history/ticker/{ticker}", "Trayectoria de un ticker"),
		func(_ context.Context, in *struct {
			Ticker   string `path:"ticker" pattern:"^[A-Za-z0-9.\\-^=]{1,15}$"`
			Analysis string `query:"analysis" pattern:"^(_[a-z]+)?$" doc:"'' = Asia + EEUU (principal), '_us' = solo EEUU"`
		}) (*struct{ Body []TdTickerHistoria }, error) {
			r, e := TickerHistory(h.S, in.Ticker, in.Analysis)
			if e != nil {
				return nil, e
			}
			return &struct{ Body []TdTickerHistoria }{r}, nil
		})
	api.Protected(h, op("td_history_universe", "/history/universe/{ticker}", "Posición de un ticker en el universo completo (pesos de Markowitz)"),
		func(_ context.Context, in *struct {
			Ticker   string `path:"ticker" pattern:"^[A-Za-z0-9.\\-^=]{1,15}$"`
			Analysis string `query:"analysis" pattern:"^(_[a-z]+)?$" doc:"'' = Asia + EEUU (principal), '_us' = solo EEUU"`
		}) (*struct{ Body []TdUniversoHistoria }, error) {
			r, e := UniverseHistory(h.S, in.Ticker, in.Analysis)
			if e != nil {
				return nil, e
			}
			return &struct{ Body []TdUniversoHistoria }{r}, nil
		})
	api.Protected(h, op("td_history_sectors", "/history/sectors", "Peso por sector en el tiempo"),
		func(_ context.Context, in *struct {
			Analysis string `query:"analysis" pattern:"^(_[a-z]+)?$" doc:"'' = Asia + EEUU (principal), '_us' = solo EEUU"`
		}) (*struct{ Body []TdSectorHistoria }, error) {
			r, e := SectorHistory(h.S, in.Analysis)
			if e != nil {
				return nil, e
			}
			return &struct{ Body []TdSectorHistoria }{r}, nil
		})
	api.Protected(h, op("td_history_momentum_leaders", "/history/momentum-leaders", "Líderes de momentum por corrida"),
		func(_ context.Context, in *struct {
			Analysis string `query:"analysis" pattern:"^(_[a-z]+)?$" doc:"'' = Asia + EEUU (principal), '_us' = solo EEUU"`
			Top      int    `query:"top" default:"5" minimum:"1" maximum:"20"`
		}) (*struct{ Body []TdMomentumLider }, error) {
			r, e := MomentumLeaders(h.S, in.Analysis, in.Top)
			if e != nil {
				return nil, e
			}
			return &struct{ Body []TdMomentumLider }{r}, nil
		})
}
