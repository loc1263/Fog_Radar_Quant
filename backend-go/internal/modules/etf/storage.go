package etf

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"fogradar/internal/core"
	"fogradar/internal/quant"
)

// Disposición en disco (estructura estándar de data/, ver core/rundir.go):
//
//	data/etf/runs/<id>/        una carpeta por corrida: etf_perf_data_<fecha>.csv, los outputs del análisis,
//	                           historial_corridas.csv y raw/ (salidas crudas del screener); run.json la marca terminada
//	data/etf/cache/            info_cache.json (info de cada ETF) y etf_perf_baseline.csv (ancla de la consistencia)
//
// Los lectores usan la última corrida terminada; el pipeline escribe en su carpeta y recién al final crea run.json,
// así que nadie lee una corrida a medias (no hacen falta candados).
const (
	key = core.ModuleKey("etf")

	// SinCorrida es el mensaje cuando no hay corrida procesada.
	SinCorrida = "Todavía no hay una corrida procesada. Ejecuta: fogradar job etf"

	// BaselineFile es el etf_perf_data de la primera corrida: el momentum mide su «consistencia» contra él.
	BaselineFile = "etf_perf_baseline.csv"
)

// CacheDir es data/etf/cache.
func CacheDir(s *core.Settings) string { return s.CacheDir(key) }

// BaselinePath es la ruta del ancla de consistencia (puede no existir todavía).
func BaselinePath(s *core.Settings) string { return filepath.Join(CacheDir(s), BaselineFile) }

// CurrentDir es la carpeta de la última corrida terminada ("" si no hay).
func CurrentDir(s *core.Settings) string { return core.LatestRun(s, key) }

var fechaDigits = regexp.MustCompile(`(\d{8})`)

// FechaDe: «etf_perf_data_20260924.csv» / «20260924» → 2026-09-24 (nil si no hay fecha válida).
func FechaDe(nombre string) *time.Time {
	m := fechaDigits.FindString(nombre)
	if m == "" {
		return nil
	}
	t, err := time.Parse("20060102", m)
	if err != nil {
		return nil
	}
	return &t
}

// Leer lee un CSV de la corrida vigente; (nil, nil) si no existe.
func Leer(s *core.Settings, nombre string) (*quant.Frame, *core.APIError) {
	dir := CurrentDir(s)
	if dir == "" {
		return nil, nil
	}
	p := filepath.Join(dir, nombre)
	if _, err := os.Stat(p); err != nil {
		return nil, nil
	}
	df, err := quant.ReadCSVFile(p)
	if err != nil {
		return nil, core.Err(http.StatusServiceUnavailable, "invalid_data", fmt.Sprintf("No se pudo leer %s: %s", nombre, "ParserError"))
	}
	return df, nil
}

// LeerRequerido es Leer pero 503 no_data si falta.
func LeerRequerido(s *core.Settings, nombre, detalle string) (*quant.Frame, *core.APIError) {
	if detalle == "" {
		detalle = SinCorrida
	}
	df, e := Leer(s, nombre)
	if e != nil {
		return nil, e
	}
	if df == nil {
		return nil, core.NoData(detalle)
	}
	return df, nil
}

// GeneradoEn es la fecha de modificación del archivo (en la zona configurada), o nil.
func GeneradoEn(s *core.Settings, nombre string) *time.Time {
	dir := CurrentDir(s)
	if dir == "" {
		return nil
	}
	st, err := os.Stat(filepath.Join(dir, nombre))
	if err != nil {
		return nil
	}
	t := st.ModTime().In(s.Loc)
	return &t
}

// HayCorridaAnterior: ¿hay una corrida terminada antes de la vigente?
func HayCorridaAnterior(s *core.Settings) bool {
	dir := CurrentDir(s)
	return dir != "" && core.PreviousRun(s, key, dir) != ""
}

// CorridasAnteriores lista las corridas terminadas anteriores a dir, la más reciente primero.
func CorridasAnteriores(s *core.Settings, dir string) []string {
	var out []string
	pasada := false
	for _, r := range core.ListRuns(s, key) {
		if pasada {
			out = append(out, r)
		}
		if filepath.Base(r) == filepath.Base(dir) {
			pasada = true
		}
	}
	return out
}

func fileOK(p string) bool { st, err := os.Stat(p); return err == nil && !st.IsDir() }

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
