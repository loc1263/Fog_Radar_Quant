package etf

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/google/uuid"

	"fogradar/internal/core"
	"fogradar/internal/quant"
)

// Disposición en disco (mismo formato de archivo que ETF_Choice/WEB/backend):
//
//	data/etf/current/    etf_perf_data_<yyyymmdd>.csv + outputs de la corrida vigente
//	data/etf/hist/       corridas archivadas con sufijo _<yyyymmdd>[_n]
//	data/etf/screener/   salidas crudas del screener (+ data_old/ con las de más de 30 días)
//
// La API solo lee current (y hist para la rotación). El pipeline escribe en una carpeta de staging y recién si
// el análisis terminó sin errores archiva la corrida vigente e instala la nueva, todo bajo el mismo lock que
// usan los GET.
const (
	key = core.ModuleKey("etf")

	// LockTimeout: segundos de espera por el lock antes de responder 503 (igual que el original).
	LockTimeout = 15 * time.Second
	// SinCorrida es el mensaje cuando no hay corrida procesada.
	SinCorrida = "Todavía no hay una corrida procesada. Ejecuta: fogradar job etf"
)

// ModuleDir es data/etf.
func ModuleDir(s *core.Settings) string { return s.ModuleDataDir(key) }

// CurrentDir es data/etf/current.
func CurrentDir(s *core.Settings) string { return filepath.Join(ModuleDir(s), "current") }

// HistDir es data/etf/hist.
func HistDir(s *core.Settings) string { return filepath.Join(ModuleDir(s), "hist") }

// ScreenerDir es data/etf/screener.
func ScreenerDir(s *core.Settings) string { return filepath.Join(ModuleDir(s), "screener") }

// ErrBusy: el backend está actualizando los datos de ETFs.
func errBusy() *core.APIError {
	return core.Err(http.StatusServiceUnavailable, "busy", "El backend está actualizando los datos de ETFs; reintenta en unos segundos.")
}

// BloqueoDatos serializa el acceso a current entre la API y el pipeline (lock de SO: si el proceso muere se
// libera solo). Si no lo obtiene en timeout devuelve 503 «busy». Devuelve la función de liberación.
func BloqueoDatos(s *core.Settings, timeout time.Duration) (func(), *core.APIError) {
	if timeout <= 0 {
		timeout = LockTimeout
	}
	dir := CurrentDir(s)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, core.Err(http.StatusInternalServerError, "internal_error", err.Error())
	}
	l := flock.New(filepath.Join(dir, ".datos.lock"))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ok, err := l.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return nil, core.Err(http.StatusInternalServerError, "internal_error", err.Error())
	}
	if !ok {
		return nil, errBusy()
	}
	return func() { _ = l.Unlock() }, nil
}

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

// Leer lee un CSV de la corrida vigente; (nil, nil) si no existe. Llamar dentro del lock.
func Leer(s *core.Settings, nombre string) (*quant.Frame, *core.APIError) {
	p := filepath.Join(CurrentDir(s), nombre)
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
	st, err := os.Stat(filepath.Join(CurrentDir(s), nombre))
	if err != nil {
		return nil
	}
	t := st.ModTime().In(s.Loc)
	return &t
}

// HayCorridaAnterior: ¿hay algún top 15 archivado?
func HayCorridaAnterior(s *core.Settings) bool {
	m, _ := filepath.Glob(filepath.Join(HistDir(s), "etf_top15_recomendados*.csv"))
	return len(m) > 0
}

// --- Archivado ---

// OutputFilesDateless son los 8 outputs sin fecha que acompañan al etf_perf_data al archivar.
var OutputFilesDateless = []string{
	"etf_top15_recomendados.csv", "etf_ranking_completo.csv", "etf_excluidos_outliers.csv",
	"etf_momentum_ranking_completo.csv", "etf_momentum_top15_score_comp.csv", "etf_categorias_ranking.csv",
	"etf_russell3000_portfolio.csv", "etf_russell3000_resumen_buckets.csv",
}

var reFechaSecuencia = regexp.MustCompile(`_(\d{8})(?:_(\d+))?\.[^.]+$`)

// FechaYSecuencia interpreta «…_<fecha>.ext» o «…_<fecha>_<n>.ext»; sin secuencia es 1. Un nombre que no sigue
// la convención da («00000000», 1) para ordenar antes que cualquier archivo con fecha real.
func FechaYSecuencia(nombre string) (string, int) {
	m := reFechaSecuencia.FindStringSubmatch(nombre)
	if m == nil {
		return "00000000", 1
	}
	seq := 1
	if m[2] != "" {
		seq, _ = strconv.Atoi(m[2])
	}
	return m[1], seq
}

func sortByFechaSeq(paths []string) {
	sort.SliceStable(paths, func(i, j int) bool {
		fi, si := FechaYSecuencia(filepath.Base(paths[i]))
		fj, sj := FechaYSecuencia(filepath.Base(paths[j]))
		if fi != fj {
			return fi < fj
		}
		return si < sj
	})
}

func destinoConSecuencia(histDir, base, fecha, ext string) string {
	d := filepath.Join(histDir, fmt.Sprintf("%s_%s%s", base, fecha, ext))
	if _, err := os.Stat(d); err != nil {
		return d
	}
	for n := 2; ; n++ {
		c := filepath.Join(histDir, fmt.Sprintf("%s_%s_%d%s", base, fecha, n, ext))
		if _, err := os.Stat(c); err != nil {
			return c
		}
	}
}

func move(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// entre discos: copiar y borrar
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
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}

// ArchivarCorrida mueve los etf_perf_data_*.csv vigentes de dataDir (y los 8 outputs sin fecha) a histDir,
// agregando sufijo _yyyymmdd y un contador de secuencia si esa fecha ya tiene una corrida archivada (la
// corrida más nueva de una fecha SIEMPRE queda archivada). Devuelve los archivos movidos.
func ArchivarCorrida(dataDir, histDir string) ([]string, error) {
	if err := os.MkdirAll(histDir, 0o755); err != nil {
		return nil, err
	}
	csvs, _ := filepath.Glob(filepath.Join(dataDir, PatronPerf))
	sort.Strings(csvs)
	if len(csvs) == 0 {
		return nil, nil
	}
	var movidos []string
	var fechas []string
	for _, c := range csvs {
		if f := fechaDigits.FindString(filepath.Base(c)); f != "" {
			fechas = append(fechas, f)
		}
	}
	sort.Strings(fechas)
	for _, c := range csvs {
		nombre := filepath.Base(c)
		if f := fechaDigits.FindString(nombre); f != "" {
			dst := destinoConSecuencia(histDir, "etf_perf_data", f, ".csv")
			if err := move(c, dst); err != nil {
				return movidos, err
			}
			movidos = append(movidos, dst)
		} else if _, err := os.Stat(filepath.Join(histDir, nombre)); err != nil {
			dst := filepath.Join(histDir, nombre)
			if err := move(c, dst); err != nil {
				return movidos, err
			}
			movidos = append(movidos, dst)
		}
	}
	if len(fechas) > 0 {
		fechaOut := fechas[len(fechas)-1]
		for _, nombre := range OutputFilesDateless {
			src := filepath.Join(dataDir, nombre)
			if _, err := os.Stat(src); err != nil {
				continue
			}
			ext := filepath.Ext(nombre)
			dst := destinoConSecuencia(histDir, strings.TrimSuffix(nombre, ext), fechaOut, ext)
			if err := move(src, dst); err != nil {
				return movidos, err
			}
			movidos = append(movidos, dst)
		}
	}
	return movidos, nil
}

// --- Staging e instalación ---

// PrepararStaging crea una carpeta temporal (en el mismo disco que current, para poder renombrar) con el CSV
// nuevo ya con su nombre final y una copia del historial vigente. No usa el directorio temporal del sistema:
// en Windows queda con una ACL sin herencia y los archivos instalados desde ahí quedan ilegibles (ver
// docs/BASELINE_GO.md §1).
func PrepararStaging(s *core.Settings, origenCSV, fechaTag string) (string, error) {
	base := ModuleDir(s)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	staging := filepath.Join(base, ".staging_"+strings.ReplaceAll(uuid.NewString(), "-", "")[:8])
	if err := os.Mkdir(staging, 0o755); err != nil {
		return "", err
	}
	if err := copyFile(origenCSV, filepath.Join(staging, "etf_perf_data_"+fechaTag+".csv")); err != nil {
		return staging, err
	}
	unlock, e := BloqueoDatos(s, 60*time.Second)
	if e != nil {
		return staging, e
	}
	defer unlock()
	if h := filepath.Join(CurrentDir(s), HistorialFile); fileOK(h) {
		if err := copyFile(h, filepath.Join(staging, HistorialFile)); err != nil {
			return staging, err
		}
	}
	return staging, nil
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

// InstalarDesdeStaging: bajo el lock, (opcional) archiva la corrida vigente en hist y mueve todo lo de
// staging a current. Devuelve los nombres instalados.
func InstalarDesdeStaging(s *core.Settings, staging string, archivar bool, log func(format string, args ...any)) ([]string, error) {
	destino := CurrentDir(s)
	if err := os.MkdirAll(destino, 0o755); err != nil {
		return nil, err
	}
	unlock, e := BloqueoDatos(s, 60*time.Second)
	if e != nil {
		return nil, e
	}
	defer unlock()
	if archivar {
		movidos, err := ArchivarCorrida(destino, HistDir(s))
		if err != nil {
			return nil, err
		}
		for _, m := range movidos {
			log("archivado: %s", filepath.Base(m))
		}
		if len(movidos) == 0 {
			log("(no había corrida vigente para archivar)")
		}
	} else if nuevo := CSVActual(staging); nuevo != "" {
		// Re-análisis: si el CSV vigente tuviera otro nombre, no dejar dos.
		viejos, _ := filepath.Glob(filepath.Join(destino, PatronPerf))
		for _, v := range viejos {
			if filepath.Base(v) != filepath.Base(nuevo) {
				_ = os.Remove(v)
			}
		}
	}
	ents, err := os.ReadDir(staging)
	if err != nil {
		return nil, err
	}
	var instalados []string
	for _, e := range ents {
		if err := os.Rename(filepath.Join(staging, e.Name()), filepath.Join(destino, e.Name())); err != nil {
			return instalados, err
		}
		instalados = append(instalados, e.Name())
	}
	return instalados, nil
}
