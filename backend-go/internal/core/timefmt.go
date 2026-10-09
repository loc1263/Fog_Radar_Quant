package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// FormatISO imprime una fecha igual que pydantic v2: sin fracción si los
// microsegundos son 0, si no 6 dígitos; "Z" para UTC y "±hh:mm" para el resto.
func FormatISO(t time.Time) string {
	s := t.Format("2006-01-02T15:04:05")
	if us := t.Nanosecond() / 1000; us != 0 {
		s += fmt.Sprintf(".%06d", us)
	}
	name, off := t.Zone()
	if off == 0 && (name == "UTC" || name == "") {
		return s + "Z"
	}
	sign := "+"
	if off < 0 {
		sign, off = "-", -off
	}
	return s + fmt.Sprintf("%s%02d:%02d", sign, off/3600, off%3600/60)
}

var isoLayouts = []string{
	"2006-01-02T15:04:05.999999999Z07:00",
	"2006-01-02T15:04:05Z07:00",
}

var isoNaiveLayouts = []string{
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05.999999999",
}

// ParseISO lee un datetime ISO 8601. Si no trae zona se interpreta en loc
// (equivale a FechaLocal de core/schemas.py).
func ParseISO(s string, loc *time.Location) (time.Time, error) {
	for _, l := range isoLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	for _, l := range isoNaiveLayouts {
		if t, err := time.ParseInLocation(l, s, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("fecha inválida: %q", s)
}

// DefaultLoc es la zona con que se interpretan las fechas sin zona al leer JSON (Settings.timezone; se fija al
// cargar la configuración). Equivale a FechaLocal de core/schemas.py.
var DefaultLoc = time.UTC

// Fecha es un time.Time que se serializa como lo hace la API Python.
type Fecha struct{ time.Time }

// MarshalJSON implementa json.Marshaler.
func (f Fecha) MarshalJSON() ([]byte, error) { return []byte(`"` + FormatISO(f.Time) + `"`), nil }

// UnmarshalJSON implementa json.Unmarshaler.
func (f *Fecha) UnmarshalJSON(b []byte) error {
	if len(b) < 2 {
		return fmt.Errorf("fecha inválida: %s", b)
	}
	t, err := ParseISO(string(b[1:len(b)-1]), DefaultLoc)
	f.Time = t
	return err
}

// Schema implementa huma.SchemaProvider: es un string date-time, no el struct embebido.
func (Fecha) Schema(huma.Registry) *huma.Schema {
	return &huma.Schema{Type: "string", Format: "date-time"}
}

// Opt es un struct que puede ser null en el JSON. Huma no admite `nullable` sobre referencias a structs, así
// que su esquema se declara como anyOf [T, null] (igual que pydantic con `T | None`).
type Opt[T any] struct{ V *T }

// Some envuelve un valor.
func Some[T any](v T) Opt[T] { return Opt[T]{&v} }

// MarshalJSON escribe null cuando no hay valor.
func (o Opt[T]) MarshalJSON() ([]byte, error) {
	if o.V == nil {
		return []byte("null"), nil
	}
	return json.Marshal(o.V)
}

// UnmarshalJSON lee null como ausente.
func (o *Opt[T]) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		o.V = nil
		return nil
	}
	o.V = new(T)
	return json.Unmarshal(b, o.V)
}

// Schema implementa huma.SchemaProvider.
func (Opt[T]) Schema(r huma.Registry) *huma.Schema {
	t := reflect.TypeFor[T]()
	return &huma.Schema{AnyOf: []*huma.Schema{r.Schema(t, true, t.Name()), {Type: "null"}}}
}
