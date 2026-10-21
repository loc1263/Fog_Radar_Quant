// Package transversal es el hub (estado + frescura + top 3 por módulo). Cada módulo aporta un Provider.
package transversal

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/api"
	"fogradar/internal/core"
	"fogradar/internal/modules"
)

// MargenCorrida: margen para que una corrida programada termine antes de considerar atrasados sus datos.
const MargenCorrida = 3 * time.Hour

// HubItem es un instrumento del top del hub.
type HubItem struct {
	Ticker string   `json:"ticker"`
	Nombre *string  `json:"nombre" nullable:"true"`
	Valor  *float64 `json:"valor" nullable:"true"`
}

// ResumenModulo es lo que cada módulo aporta al hub: fecha de sus datos y su top 3.
type ResumenModulo struct {
	DatosAl  *core.Fecha `json:"datos_al" nullable:"true"`
	Metrica  string      `json:"metrica"`
	Unidad   string      `json:"unidad" enum:"fraccion,numero" doc:"Cómo formatear valor: fraccion → porcentaje (0.12 = 12 %), numero → tal cual."`
	Top      []HubItem   `json:"top"`
	Universo *int64      `json:"universo" nullable:"true" doc:"Cuántos instrumentos cubre la última corrida (acciones, ETFs…)."`
}

// HubModulo es el estado de un módulo en el hub.
type HubModulo struct {
	DatosAl   *core.Fecha `json:"datos_al" nullable:"true"`
	Metrica   string      `json:"metrica"`
	Unidad    string      `json:"unidad" enum:"fraccion,numero"`
	Top       []HubItem   `json:"top"`
	Universo  *int64      `json:"universo" nullable:"true"`
	Key       string      `json:"key"`
	Titulo    string      `json:"titulo"`
	Frescura  string      `json:"frescura" enum:"al_dia,atrasado,sin_datos,error"`
	EdadHoras *float64    `json:"edad_horas" nullable:"true"`
	Programa  *string     `json:"programa" nullable:"true" doc:"Cuándo se actualiza en producción (p. ej. \"de lunes a viernes a las 18:00\")."`
	Proxima   *core.Fecha `json:"proxima" nullable:"true"`
	Mensaje   *string     `json:"mensaje" nullable:"true" doc:"Motivo si no hay datos o no se pudieron leer."`
	UltimoJob api.OptJob  `json:"ultimo_job"`
}

// HubResponse es la respuesta de /hub.
type HubResponse struct {
	GeneradoEn core.Fecha  `json:"generado_en"`
	Modulos    []HubModulo `json:"modulos"`
}

// Provider es lo que cada módulo expone al hub.
type Provider interface {
	ResumenHub(s *core.Settings) (*ResumenModulo, error)
}

var (
	provMu    sync.RWMutex
	providers = map[string]Provider{}
)

// RegisterProvider registra el aporte de un módulo.
func RegisterProvider(key string, p Provider) {
	provMu.Lock()
	defer provMu.Unlock()
	providers[key] = p
}

func provider(key string) Provider {
	provMu.RLock()
	defer provMu.RUnlock()
	return providers[key]
}

// isNoData dice si el error es el 503 «no_data» de un módulo sin corrida.
func isNoData(err error) bool {
	var ae *core.APIError
	return errors.As(err, &ae) && ae.Code == "no_data"
}

// Frescura: «al_dia» si los datos son posteriores a la última corrida programada que ya debería
// haber terminado; «atrasado» si no. Sin programa: al día.
func Frescura(modulo string, datosAl *time.Time, ahora time.Time) string {
	if datosAl == nil {
		return "sin_datos"
	}
	p, ok := core.ProgramaDe(modulo)
	if !ok {
		return "al_dia"
	}
	esperada := p.Anterior(ahora.Add(-MargenCorrida))
	if datosAl.Before(esperada) {
		return "atrasado"
	}
	return "al_dia"
}

func round1(x float64) float64 {
	v, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 1, 64), 64)
	return v
}

// Hub arma el estado de los 4 módulos.
func Hub(s *core.Settings, jobs *core.JobRegistry, ahora time.Time) HubResponse {
	out := HubResponse{GeneradoEn: core.Fecha{Time: ahora}, Modulos: make([]HubModulo, 0, len(modules.All))}
	for _, m := range modules.All {
		out.Modulos = append(out.Modulos, hubModulo(s, m, jobs, ahora))
	}
	return out
}

func hubModulo(s *core.Settings, info modules.Info, jobs *core.JobRegistry, ahora time.Time) HubModulo {
	key := string(info.Key)
	h := HubModulo{Key: key, Titulo: info.Title, Unidad: "numero", Top: []HubItem{},
		UltimoJob: api.OptJob{JobRecord: jobs.Latest(key)}}
	if p, ok := core.ProgramaDe(key); ok {
		d := p.Descripcion()
		h.Programa = &d
		h.Proxima = &core.Fecha{Time: p.Proxima(ahora)}
	}
	prov := provider(key)
	if prov == nil {
		msg := "Módulo sin proveedor de hub."
		h.Frescura, h.Mensaje = "error", &msg
		return h
	}
	r, err := prov.ResumenHub(s)
	if err != nil {
		if isNoData(err) {
			msg := err.Error()
			h.Frescura, h.Mensaje = "sin_datos", &msg
			return h
		}
		msg := "Los datos del módulo no se pudieron leer."
		if !s.IsProd() {
			msg = err.Error()
		}
		h.Frescura, h.Mensaje = "error", &msg
		return h
	}
	h.Metrica, h.Unidad, h.Top, h.Universo = r.Metrica, r.Unidad, r.Top, r.Universo
	if h.Top == nil {
		h.Top = []HubItem{}
	}
	var datosAl *time.Time
	if r.DatosAl != nil {
		t := r.DatosAl.In(s.Loc)
		datosAl = &t
		h.DatosAl = &core.Fecha{Time: t}
		e := round1(ahora.Sub(t).Hours())
		h.EdadHoras = &e
	}
	h.Frescura = Frescura(key, datosAl, ahora)
	return h
}

// Register agrega /api/hub.
func Register(h *api.Host) {
	api.Protected(h, huma.Operation{
		OperationID: "get_hub", Method: http.MethodGet, Path: "/api/hub",
		Summary: "Estado, frescura y top 3 de cada módulo", Tags: []string{"Transversal"},
	}, func(context.Context, *struct{}) (*struct{ Body HubResponse }, error) {
		return &struct{ Body HubResponse }{Hub(h.S, h.Jobs, time.Now().In(h.S.Loc))}, nil
	})
}
