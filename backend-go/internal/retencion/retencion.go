// Package retencion borra de data/ las corridas de más de N días (por defecto 7): el sitio solo muestra la última
// corrida de cada módulo, así que no hace falta guardar más historia. Trabaja sobre la estructura estándar
// (data/<modulo>/runs/<id>/, ver core/rundir.go): borra carpetas de corrida enteras y nunca la más reciente,
// aunque sea más antigua que el límite. Las corridas sin run.json (fallidas o interrumpidas) se borran pasado un día.
// data/<modulo>/cache/ no se toca: son cachés y series que evitan miles de requests a Yahoo.
package retencion

import (
	"os"
	"path/filepath"
	"time"

	"fogradar/internal/core"
)

// DiasPorDefecto es la ventana de retención (RETENCION_DIAS).
const DiasPorDefecto = 7

// Resultado de una poda.
type Resultado struct {
	Eliminados []string
	Bytes      int64
}

// Podar aplica la retención a todos los módulos.
func Podar(s *core.Settings, dias int, ahora time.Time, simular bool) (Resultado, error) {
	var out Resultado
	for _, m := range []string{"cl_stocks", "us_stocks", "etf", "tendencia"} {
		r, err := PodarModulo(s, m, dias, ahora, simular)
		out.Eliminados = append(out.Eliminados, r.Eliminados...)
		out.Bytes += r.Bytes
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// PodarModulo aplica la retención a un módulo. dias <= 0 la desactiva.
func PodarModulo(s *core.Settings, modulo string, dias int, ahora time.Time, simular bool) (Resultado, error) {
	var out Resultado
	if dias <= 0 {
		return out, nil
	}
	m := core.ModuleKey(modulo)
	ahora = ahora.In(s.Loc)
	hoy := time.Date(ahora.Year(), ahora.Month(), ahora.Day(), 0, 0, 0, 0, ahora.Location())
	limite := hoy.AddDate(0, 0, -(dias - 1)) // los últimos `dias` días, hoy incluido

	ents, err := os.ReadDir(s.RunsDir(m))
	if err != nil {
		return out, nil // sin carpeta de corridas: nada que podar
	}
	vigente := core.LatestRun(s, m)
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(s.RunsDir(m), e.Name())
		if dir == vigente {
			continue
		}
		t := core.RunTime(dir, s.Loc)
		if t == nil {
			continue // no es una carpeta de corrida: no se toca
		}
		viejaPorFecha := t.Before(limite)
		incompleta := !core.IsFinished(dir) && ahora.Sub(*t) > 24*time.Hour
		if !viejaPorFecha && !incompleta {
			continue
		}
		out.Bytes += tamano(dir)
		out.Eliminados = append(out.Eliminados, dir)
		if !simular {
			if err := os.RemoveAll(dir); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}

func tamano(path string) int64 {
	var n int64
	_ = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, e := d.Info(); e == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}
