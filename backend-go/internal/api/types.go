package api

import (
	"encoding/json"
	"reflect"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/core"
)

// OptJob es un JobRecord que puede ser null. Huma no admite `nullable` sobre referencias a
// structs, así que se declara como anyOf [JobRecord, null].
type OptJob struct{ *core.JobRecord }

// MarshalJSON escribe null cuando no hay job.
func (o OptJob) MarshalJSON() ([]byte, error) {
	if o.JobRecord == nil {
		return []byte("null"), nil
	}
	return json.Marshal(o.JobRecord)
}

// Schema implementa huma.SchemaProvider.
func (OptJob) Schema(r huma.Registry) *huma.Schema {
	return &huma.Schema{AnyOf: []*huma.Schema{
		r.Schema(reflect.TypeOf(core.JobRecord{}), true, "JobRecord"),
		{Type: "null"},
	}}
}
