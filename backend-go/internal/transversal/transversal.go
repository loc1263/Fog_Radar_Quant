// Package transversal es el hub (estado + frescura + top 3 por módulo) y el buscador global de
// ticker. Cada módulo aporta un Provider.
package transversal

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/text/unicode/norm"

	"fogradar/internal/api"
	"fogradar/internal/core"
	"fogradar/internal/modules"
)

// MargenCorrida: margen para que una corrida programada termine antes de considerar atrasados sus datos.
const MargenCorrida = 3 * time.Hour

// MaxResultados del buscador.
const MaxResultados = 50

const ttlIndice = 60 * time.Second

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

// EntradaBusqueda es un instrumento en el índice de búsqueda de un módulo.
type EntradaBusqueda struct {
	Ticker   string   `json:"ticker"`
	Nombre   *string  `json:"nombre" nullable:"true"`
	Posicion *int64   `json:"posicion" nullable:"true" doc:"Posición en el ranking del módulo (1 = mejor), si aplica."`
	Total    *int64   `json:"total" nullable:"true"`
	Valor    *float64 `json:"valor" nullable:"true"`
	Metrica  string   `json:"metrica"`
	Unidad   string   `json:"unidad" enum:"fraccion,numero"`
	Contexto *string  `json:"contexto" nullable:"true" doc:"Dónde aparece (p. ej. \"Top 15\", \"Portafolio Asia + EEUU\")."`
}

// ResultadoBusqueda es una entrada con su módulo.
type ResultadoBusqueda struct {
	EntradaBusqueda
	Modulo       string `json:"modulo"`
	TituloModulo string `json:"titulo_modulo"`
	Exacto       bool   `json:"exacto" doc:"El ticker coincide exactamente con la consulta (va primero)."`
}

// BusquedaResponse es la respuesta de /buscar.
type BusquedaResponse struct {
	Consulta   string              `json:"consulta"`
	Resultados []ResultadoBusqueda `json:"resultados"`
}

// Provider es lo que cada módulo expone al hub y al buscador.
type Provider interface {
	ResumenHub(s *core.Settings) (*ResumenModulo, error)
	IndiceBusqueda(s *core.Settings) ([]EntradaBusqueda, error)
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

// --- Buscador -------------------------------------------------------------------------

type cacheKey struct{ dir, modulo string }

type cacheVal struct {
	at      time.Time
	entries []EntradaBusqueda
}

var (
	cacheMu sync.Mutex
	cache   = map[cacheKey]cacheVal{}
)

// LimpiarCache vacía el índice de búsqueda (tests).
func LimpiarCache() {
	cacheMu.Lock()
	cache = map[cacheKey]cacheVal{}
	cacheMu.Unlock()
}

func indice(s *core.Settings, modulo string) []EntradaBusqueda {
	k := cacheKey{s.DataDir, modulo}
	cacheMu.Lock()
	if v, ok := cache[k]; ok && time.Since(v.at) < ttlIndice {
		cacheMu.Unlock()
		return v.entries
	}
	cacheMu.Unlock()
	var entries []EntradaBusqueda
	if p := provider(modulo); p != nil {
		if e, err := p.IndiceBusqueda(s); err == nil {
			entries = e
		}
	}
	cacheMu.Lock()
	cache[k] = cacheVal{time.Now(), entries}
	cacheMu.Unlock()
	return entries
}

// normalizar quita tildes, pasa a minúsculas y colapsa espacios.
func normalizar(t string) string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(t) {
		if r < unicode.MaxASCII {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// baseTicker: «VAPORES.SN» → «vapores»; «^VIX» → «vix».
func baseTicker(t string) string {
	base, _, _ := strings.Cut(t, ".")
	return normalizar(strings.TrimLeft(base, "^"))
}

// Buscar busca por ticker (exacto, sin sufijo de bolsa o por prefijo) y por nombre (contiene).
// Orden: coincidencia exacta de ticker, luego prefijo, luego nombre; dentro de cada grupo, por
// módulo y posición en su ranking.
func Buscar(s *core.Settings, consulta string, limite int) BusquedaResponse {
	q := normalizar(consulta)
	out := BusquedaResponse{Consulta: consulta, Resultados: []ResultadoBusqueda{}}
	if q == "" {
		return out
	}
	type item struct {
		key [3]int
		r   ResultadoBusqueda
	}
	var items []item
	for ord, info := range modules.All {
		for _, e := range indice(s, string(info.Key)) {
			t := normalizar(e.Ticker)
			var grupo int
			switch {
			case t == q || baseTicker(e.Ticker) == q:
				grupo = 0
			case strings.HasPrefix(t, q):
				grupo = 1
			case len(q) >= 3 && e.Nombre != nil && strings.Contains(normalizar(*e.Nombre), q):
				grupo = 2
			default:
				continue
			}
			pos := 1_000_000
			if e.Posicion != nil {
				pos = int(*e.Posicion)
			}
			items = append(items, item{[3]int{grupo, ord, pos}, ResultadoBusqueda{
				EntradaBusqueda: e, Modulo: string(info.Key), TituloModulo: info.Title, Exacto: grupo == 0}})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].key, items[j].key
		for k := range a {
			if a[k] != b[k] {
				return a[k] < b[k]
			}
		}
		return false
	})
	for i := 0; i < len(items) && i < limite; i++ {
		out.Resultados = append(out.Resultados, items[i].r)
	}
	return out
}

// Register agrega /api/hub y /api/buscar.
func Register(h *api.Host) {
	api.Protected(h, huma.Operation{
		OperationID: "get_hub", Method: http.MethodGet, Path: "/api/hub",
		Summary: "Estado, frescura y top 3 de cada módulo", Tags: []string{"Transversal"},
	}, func(context.Context, *struct{}) (*struct{ Body HubResponse }, error) {
		return &struct{ Body HubResponse }{Hub(h.S, h.Jobs, time.Now().In(h.S.Loc))}, nil
	})

	op := huma.Operation{
		OperationID: "get_buscar", Method: http.MethodGet, Path: "/api/buscar",
		Summary: "Buscador global: en qué módulos aparece un ticker y con qué puntaje", Tags: []string{"Transversal"},
	}
	api.ProtectedWith(h, op, func(_ context.Context, in *struct {
		Q      string `query:"q" required:"true" minLength:"1" maxLength:"40" doc:"Ticker o parte del nombre"`
		Limite int    `query:"limite" default:"20" minimum:"1" maximum:"50"`
	}) (*struct{ Body BusquedaResponse }, error) {
		return &struct{ Body BusquedaResponse }{Buscar(h.S, in.Q, in.Limite)}, nil
	}, h.Extra("120/minute", "buscar"))
}
