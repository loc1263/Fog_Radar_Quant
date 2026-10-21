// Package jobs ejecuta los pipelines batch (equivale a fog_radar/jobs): registro de pipelines por
// módulo, runner en secuencia bajo el lock global, reintentos y aviso por webhook.
//
// Los pipelines son batch por CLI (`fogradar job <modulo>`) y nunca corren dentro de un request.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"fogradar/internal/core"
	"fogradar/internal/yahoo"
)

// Env es lo que recibe un pipeline: configuración, el cliente de Yahoo común (con sus contadores) y un logger.
type Env struct {
	S       *core.Settings
	Traffic *yahoo.Traffic
	Yahoo   yahoo.Source
	Log     *slog.Logger
}

// OptKind es el tipo de una opción del CLI.
type OptKind int

const (
	// Flag es una bandera sin valor (--solo-scoring → true).
	Flag OptKind = iota
	// Int lleva un entero validado contra Min/Max.
	Int
	// Float lleva un decimal validado contra Min/Max.
	Float
)

// Option es una opción del CLI de un pipeline.
type Option struct {
	Flag     string // «--solo-scoring»
	Help     string
	Kind     OptKind
	Min, Max float64
	Key      string // nombre en el mapa de opciones; por defecto la bandera sin «--» ni guiones
}

// Dest es la clave de la opción en el mapa que recibe Run.
func (o Option) Dest() string {
	if o.Key != "" {
		return o.Key
	}
	d := ""
	for _, r := range o.Flag[2:] {
		if r == '-' {
			r = '_'
		}
		d += string(r)
	}
	return d
}

// Convert parsea y valida el valor de una opción con valor.
func (o Option) Convert(text string) (any, error) {
	switch o.Kind {
	case Int:
		v, err := strconv.Atoi(text)
		if err != nil {
			return nil, fmt.Errorf("%s: '%s' no es un int", o.Flag, text)
		}
		if float64(v) < o.Min || float64(v) > o.Max {
			return nil, fmt.Errorf("%s: %d fuera de rango [%g, %g]", o.Flag, v, o.Min, o.Max)
		}
		return v, nil
	case Float:
		v, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: '%s' no es un float", o.Flag, text)
		}
		if v < o.Min || v > o.Max {
			return nil, fmt.Errorf("%s: %g fuera de rango [%g, %g]", o.Flag, v, o.Min, o.Max)
		}
		return v, nil
	}
	return true, nil
}

// Pipeline de un módulo. Run devuelve un resumen para el registro de jobs.
type Pipeline struct {
	Run     func(ctx context.Context, env *Env, opts map[string]any) (string, error)
	Options []Option
}

var pipelines = map[string]Pipeline{}

// Register registra el pipeline de un módulo.
func Register(module string, p Pipeline) { pipelines[module] = p }

// Get devuelve el pipeline de un módulo.
func Get(module string) (Pipeline, bool) { p, ok := pipelines[module]; return p, ok }

// Backfill es la opción común de todos los módulos: reconstruye history.db desde los archivos.
var Backfill = Option{Flag: "--backfill", Help: "Solo carga en history.db las corridas ya guardadas en disco."}

// OrdenTodos: los diarios primero y ETF (el más largo, ~30 min) al final.
var OrdenTodos = []string{"cl_stocks", "us_stocks", "tendencia", "etf"}

// OpcionesSinCache hace que un módulo ignore sus cachés y pida todo de nuevo a Yahoo (--no-cache). cl_stocks y
// us_stocks no tienen caché: siempre descargan, así que no necesitan opciones.
var OpcionesSinCache = map[string]map[string]any{
	"etf":       {"refrescar_info": true},
	"tendencia": {"force_refresh": true, "force_discovery": true},
}

// OpcionesOffline corre un módulo sin descargar de Yahoo, sobre los datos ya descargados.
// us_stocks no tiene modo sin descarga: con --offline se omite.
var OpcionesOffline = map[string]map[string]any{
	"cl_stocks": {"solo_scoring": true},
	"etf":       {"solo_analisis": true},
	"tendencia": {"skip_fetch": true, "skip_discovery": true},
}
