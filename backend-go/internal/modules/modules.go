// Package modules registra los 4 módulos del sitio (equivale a fog_radar/modules/__init__.py).
package modules

import "fogradar/internal/core"

// Info describe un módulo.
type Info struct {
	Key            core.ModuleKey
	Prefix         string // bajo /api
	Title          string
	SourceProject  string
	MigrationPhase int
	// Migrated es true cuando su pipeline Python está portado a Fog Radar (campo `migrated`
	// de /status). Es independiente de qué lenguaje sirve la API.
	Migrated bool
}

// All en el mismo orden que Python.
var All = []Info{
	{"cl_stocks", "/cl", "Acciones Chile (IPSA)", "Stock_Choice_Nacional", 3, true},
	{"us_stocks", "/us", "Acciones EEUU", "Stock_Choice_EEUU", 4, true},
	{"etf", "/etf", "ETFs", "ETF_Choice", 5, true},
	{"tendencia", "/tendencia", "Tendencia Asia + EEUU", "Stock_Tendencia", 6, true},
}

// ByKey busca un módulo por clave.
func ByKey(key string) (Info, bool) {
	for _, m := range All {
		if string(m.Key) == key {
			return m, true
		}
	}
	return Info{}, false
}
