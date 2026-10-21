// Package retencion borra de data/ lo que tiene más de N días (por defecto 7): el sitio solo muestra la
// última corrida de cada módulo, así que no hace falta guardar más historia. Nunca borra la entrada
// más reciente de cada familia de archivos, aunque sea más antigua que el límite.
package retencion

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"fogradar/internal/core"
)

// DiasPorDefecto es la ventana de retención (RETENCION_DIAS).
const DiasPorDefecto = 7

// fechaRe encuentra la primera fecha (yyyymmdd o yyyy-mm-dd) del nombre.
var fechaRe = regexp.MustCompile(`(\d{4})-?(\d{2})-?(\d{2})`)

// Resultado de una poda.
type Resultado struct {
	Eliminados []string
	Bytes      int64
}

// regla describe una carpeta con entradas fechadas en su nombre.
type regla struct {
	modulo string
	sub    string // carpeta relativa a data/<modulo>
	// conservarMasAntiguo: prefijo de la familia cuyo archivo más antiguo se conserva como ancla
	// (el momentum de ETF mide la consistencia contra el etf_perf_data más antiguo de hist/).
	conservarMasAntiguo string
}

var reglas = []regla{
	{modulo: "cl_stocks", sub: "."},
	{modulo: "us_stocks", sub: "runs"},
	{modulo: "etf", sub: "hist", conservarMasAntiguo: "etf_perf_data_"},
	{modulo: "etf", sub: "screener"},
	{modulo: "tendencia", sub: "output"},
	{modulo: "tendencia", sub: "macro"},
	{modulo: "tendencia", sub: "universe"},
}

// dirsCompletos son carpetas que se eliminan por entero (salidas crudas de más de 30 días).
var dirsCompletos = []struct{ modulo, sub string }{{"etf", "screener/data_old"}}

type entrada struct {
	path   string
	nombre string
	fecha  time.Time
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
	ahora = ahora.In(s.Loc)
	hoy := time.Date(ahora.Year(), ahora.Month(), ahora.Day(), 0, 0, 0, 0, ahora.Location())
	limite := hoy.AddDate(0, 0, -(dias - 1)) // los últimos `dias` días, hoy incluido
	base := s.ModuleDataDir(core.ModuleKey(modulo))

	for _, r := range reglas {
		if r.modulo != modulo {
			continue
		}
		dir := filepath.Join(base, filepath.FromSlash(r.sub))
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue // carpeta inexistente: nada que podar
		}
		porFamilia := map[string][]entrada{}
		for _, e := range ents {
			nombre := e.Name()
			m := fechaRe.FindStringSubmatchIndex(nombre)
			if m == nil {
				continue // sin fecha en el nombre (cachés, archivos de estado): no se tocan
			}
			y, _ := strconv.Atoi(nombre[m[2]:m[3]])
			mo, _ := strconv.Atoi(nombre[m[4]:m[5]])
			d, _ := strconv.Atoi(nombre[m[6]:m[7]])
			fam := nombre[:m[0]]
			if !e.IsDir() {
				fam += filepath.Ext(nombre)
			}
			porFamilia[fam] = append(porFamilia[fam], entrada{
				path: filepath.Join(dir, nombre), nombre: nombre,
				fecha: time.Date(y, time.Month(mo), d, 0, 0, 0, 0, ahora.Location()),
			})
		}
		for fam, lista := range porFamilia {
			sort.Slice(lista, func(i, j int) bool { return lista[i].nombre < lista[j].nombre })
			for i, e := range lista {
				masReciente := i == len(lista)-1
				ancla := r.conservarMasAntiguo != "" && strings.HasPrefix(fam, r.conservarMasAntiguo) && i == 0
				if !e.fecha.Before(limite) || masReciente || ancla {
					continue
				}
				if err := eliminar(e.path, simular, &out); err != nil {
					return out, err
				}
			}
		}
	}
	for _, d := range dirsCompletos {
		if d.modulo != modulo {
			continue
		}
		p := filepath.Join(base, filepath.FromSlash(d.sub))
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if err := eliminar(p, simular, &out); err != nil {
			return out, err
		}
	}
	return out, nil
}

func eliminar(path string, simular bool, out *Resultado) error {
	out.Bytes += tamano(path)
	out.Eliminados = append(out.Eliminados, path)
	if simular {
		return nil
	}
	return os.RemoveAll(path)
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
