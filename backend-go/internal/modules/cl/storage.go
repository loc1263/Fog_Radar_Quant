package cl

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fogradar/internal/core"
	"fogradar/internal/quant"
)

// Archivos de cada corrida (data/cl_stocks/runs/<id>/):
//
//	indicadores.csv: salida del extractor
//	ranking.csv:     salida del scoring
const (
	ArchivoIndicadores = "indicadores"
	ArchivoRanking     = "ranking"
)

// ColumnasRequeridasRanking son las que debe traer un ranking para servirse.
var ColumnasRequeridasRanking = func() []string {
	c := append([]string{"Ranking"}, ColumnasIdentidad...)
	c = append(c, "Score Compuesto")
	for _, p := range Pilares {
		c = append(c, "Score "+p)
	}
	return append(c, "Métricas Disponibles", "Métricas Totales", "Grupo Financiero Duplicado")
}()

// Loaded es un CSV cargado.
type Loaded struct {
	Path       string
	DF         *quant.Frame
	GeneradoEn *time.Time
}

// LatestFile devuelve el CSV `nombre` de la corrida terminada más reciente que lo tenga ("" si no hay).
func LatestFile(s *core.Settings, nombre string) string {
	for _, run := range core.ListRuns(s, key) {
		if p := filepath.Join(run, nombre+".csv"); fileExists(p) {
			return p
		}
	}
	return ""
}

func fileExists(p string) bool { st, err := os.Stat(p); return err == nil && !st.IsDir() }

// desde identifica un archivo cargado: «<corrida>/<archivo>».
func desde(path string) string { return filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path) }

func invalidData(detail string) *core.APIError {
	return core.Err(http.StatusServiceUnavailable, "invalid_data", detail)
}

func cargar(s *core.Settings, nombre string, requeridas []string, que string) (*Loaded, *core.APIError) {
	path := LatestFile(s, nombre)
	if path == "" {
		return nil, core.NoData(fmt.Sprintf("Todavía no hay %s del IPSA. Ejecuta el pipeline: fogradar job cl_stocks", que))
	}
	df, err := quant.ReadCSVFile(path)
	if err != nil {
		return nil, invalidData(fmt.Sprintf("No se pudo leer %s: %s", filepath.Base(path), "ParserError"))
	}
	var falt []string
	for _, c := range requeridas {
		if !df.Has(c) {
			falt = append(falt, c)
		}
	}
	if len(falt) > 0 {
		return nil, invalidData(fmt.Sprintf("%s no tiene las columnas: %s", filepath.Base(path), strings.Join(falt, ", ")))
	}
	return &Loaded{path, df, core.RunTime(filepath.Dir(path), s.Loc)}, nil
}

// CargarRanking lee el último ranking.
func CargarRanking(s *core.Settings) (*Loaded, *core.APIError) {
	return cargar(s, ArchivoRanking, ColumnasRequeridasRanking, "un ranking")
}

// CargarIndicadores lee el último CSV de indicadores.
func CargarIndicadores(s *core.Settings) (*Loaded, *core.APIError) {
	return cargar(s, ArchivoIndicadores, ColumnasIdentidad, "indicadores")
}

func escribirAtomico(destino string, write func(*os.File) error) error {
	tmp := filepath.Join(filepath.Dir(destino), "."+filepath.Base(destino)+".tmp")
	f, err := os.Create(tmp) // en la misma carpeta de datos: hereda su ACL (ver docs/BASELINE_GO.md §1)
	if err != nil {
		return err
	}
	if err := write(f); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, destino)
}

// Guardar escribe <dir>/<nombre>.csv (el que lee la API) y devuelve su ruta.
func Guardar(df *quant.Frame, dir, nombre string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := filepath.Join(dir, nombre)
	err := escribirAtomico(base+".csv", func(f *os.File) error {
		w := bufio.NewWriter(f)
		if err := df.WriteCSV(w); err != nil {
			return err
		}
		return w.Flush()
	})
	return base + ".csv", err
}
