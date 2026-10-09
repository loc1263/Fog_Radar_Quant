package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/core"
	"fogradar/internal/modules"
)

// HealthResponse es la salud del servicio (público).
type HealthResponse struct {
	Status  string `json:"status" enum:"ok" default:"ok" required:"false"`
	App     string `json:"app"`
	Version string `json:"version"`
	Env     string `json:"env"`
}

// ModuleStatus es el estado de un módulo (datos presentes + último job).
type ModuleStatus struct {
	Key            string `json:"key"`
	Title          string `json:"title"`
	APIPrefix      string `json:"api_prefix"`
	SourceProject  string `json:"source_project"`
	Migrated       bool   `json:"migrated" doc:"True cuando el pipeline del módulo ya está portado a Fog Radar."`
	MigrationPhase int    `json:"migration_phase"`
	DataAvailable  bool   `json:"data_available"`
	LatestJob      OptJob `json:"latest_job" required:"false"`
	LastSuccess    OptJob `json:"last_success" required:"false"`
}

// SystemStatus es el estado de los 4 módulos.
type SystemStatus struct {
	App     string         `json:"app"`
	Version string         `json:"version"`
	Env     string         `json:"env"`
	Modules []ModuleStatus `json:"modules"`
}

// ProgramaInfo es una corrida programada del cron de producción.
type ProgramaInfo struct {
	Key         string     `json:"key"`
	Titulo      string     `json:"titulo"`
	Modulos     []string   `json:"modulos"`
	Cron        string     `json:"cron"`
	Descripcion string     `json:"descripcion"`
	Zona        string     `json:"zona"`
	Proxima     core.Fecha `json:"proxima"`
}

func (h *Host) moduleStatus(m modules.Info) ModuleStatus {
	return ModuleStatus{
		Key:            string(m.Key),
		Title:          m.Title,
		APIPrefix:      "/api" + m.Prefix,
		SourceProject:  m.SourceProject,
		Migrated:       m.Migrated,
		MigrationPhase: m.MigrationPhase,
		DataAvailable:  hasData(h.S.ModuleDataDir(m.Key)),
		LatestJob:      OptJob{h.Jobs.Latest(string(m.Key))},
		LastSuccess:    OptJob{h.Jobs.LastSuccess(string(m.Key))},
	}
}

func registerSystem(h *Host) {
	Public(h, huma.Operation{
		OperationID: "health", Method: http.MethodGet, Path: "/api/health",
		Summary: "Salud del servicio (público)", Tags: []string{"Sistema"},
	}, func(context.Context, *struct{}) (*struct{ Body HealthResponse }, error) {
		return &struct{ Body HealthResponse }{HealthResponse{
			Status: "ok", App: h.S.AppName, Version: core.Version, Env: h.S.Env}}, nil
	})

	Protected(h, huma.Operation{
		OperationID: "system_status", Method: http.MethodGet, Path: "/api/status",
		Summary: "Estado de los 4 módulos", Tags: []string{"Sistema"},
	}, func(context.Context, *struct{}) (*struct{ Body SystemStatus }, error) {
		out := SystemStatus{App: h.S.AppName, Version: core.Version, Env: h.S.Env,
			Modules: make([]ModuleStatus, 0, len(modules.All))}
		for _, m := range modules.All {
			out.Modules = append(out.Modules, h.moduleStatus(m))
		}
		return &struct{ Body SystemStatus }{out}, nil
	})

	Protected(h, huma.Operation{
		OperationID: "jobs_latest", Method: http.MethodGet, Path: "/api/jobs",
		Summary: "Último job de cada módulo", Tags: []string{"Sistema"},
	}, func(context.Context, *struct{}) (*struct{ Body map[string]OptJob }, error) {
		out := make(map[string]OptJob, len(modules.All))
		for _, m := range modules.All {
			out[string(m.Key)] = OptJob{h.Jobs.Latest(string(m.Key))}
		}
		return &struct{ Body map[string]OptJob }{out}, nil
	})

	Protected(h, huma.Operation{
		OperationID: "jobs_programas", Method: http.MethodGet, Path: "/api/jobs/programas",
		Summary: "Programación de las corridas batch", Tags: []string{"Sistema"},
	}, func(context.Context, *struct{}) (*struct{ Body []ProgramaInfo }, error) {
		ahora := time.Now().In(h.S.Loc)
		out := make([]ProgramaInfo, 0, len(core.Programas))
		for _, p := range core.Programas {
			out = append(out, ProgramaInfo{
				Key: p.Key, Titulo: p.Titulo, Modulos: p.Modulos, Cron: p.Cron(),
				Descripcion: p.Descripcion(), Zona: h.S.Timezone, Proxima: core.Fecha{Time: p.Proxima(ahora)},
			})
		}
		return &struct{ Body []ProgramaInfo }{out}, nil
	})

	Protected(h, huma.Operation{
		OperationID: "jobs_history", Method: http.MethodGet, Path: "/api/jobs/{module}",
		Summary: "Historial de jobs de un módulo", Tags: []string{"Sistema"},
	}, func(_ context.Context, in *struct {
		Module string `path:"module"`
		Limit  int    `query:"limit" default:"20" minimum:"1" maximum:"200"`
	}) (*struct{ Body []core.JobRecord }, error) {
		if _, ok := modules.ByKey(in.Module); !ok {
			return nil, core.Err(http.StatusNotFound, "unknown_module", "Módulo desconocido: "+in.Module)
		}
		return &struct{ Body []core.JobRecord }{h.Jobs.History(in.Module, in.Limit)}, nil
	})

	// GET /api/<prefijo>/status de cada módulo.
	for _, m := range modules.All {
		Protected(h, huma.Operation{
			OperationID: string(m.Key) + "_status", Method: http.MethodGet, Path: "/api" + m.Prefix + "/status",
			Summary: "Estado del módulo " + m.Title, Tags: []string{modTag(m.Key)},
		}, func(context.Context, *struct{}) (*struct{ Body ModuleStatus }, error) {
			return &struct{ Body ModuleStatus }{h.moduleStatus(m)}, nil
		})
	}
}

func modTag(k core.ModuleKey) string {
	switch k {
	case "cl_stocks":
		return "Chile (IPSA)"
	case "us_stocks":
		return "EEUU"
	case "etf":
		return "ETF"
	}
	return "Tendencia"
}
