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
		DataAvailable:  core.LatestRun(h.S, m.Key) != "",
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

	Public(h, huma.Operation{
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

	Public(h, huma.Operation{
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
}
