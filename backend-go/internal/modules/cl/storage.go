package cl

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"fogradar/internal/core"
	"fogradar/internal/quant"
)

// Archivos de datos:
//
//	indicadores_acciones_<YYYYMMDD_HHMMSS>.csv (+ .xlsx): salida del extractor
//	ranking_acciones_<YYYYMMDD_HHMMSS>.csv     (+ .xlsx): salida del scoring
const (
	PatronIndicadores = "indicadores_acciones_*.csv"
	PatronRanking     = "ranking_acciones_*.csv"
)

var tsRe = regexp.MustCompile(`_(\d{8}_\d{6})\.csv$`)

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

// DataDir es data/cl_stocks.
func DataDir(s *core.Settings) string { return s.ModuleDataDir(key) }

// LatestFile devuelve el archivo más reciente (por nombre, que lleva el timestamp) que calza con el patrón.
func LatestFile(dir, pattern string) string {
	all := ListFiles(dir, pattern)
	if len(all) == 0 {
		return ""
	}
	return all[0]
}

// ListFiles lista los archivos que calzan, del más reciente al más antiguo (por nombre).
func ListFiles(dir, pattern string) []string {
	m, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return nil
	}
	var files []string
	for _, p := range m {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			files = append(files, p)
		}
	}
	sort.Slice(files, func(i, j int) bool { return filepath.Base(files[i]) > filepath.Base(files[j]) })
	return files
}

// TimestampDe es la fecha/hora codificada en el nombre del archivo (hora local de loc).
func TimestampDe(path string, loc *time.Location) *time.Time {
	m := tsRe.FindStringSubmatch(filepath.Base(path))
	if m == nil {
		return nil
	}
	t, err := time.ParseInLocation("20060102_150405", m[1], loc)
	if err != nil {
		return nil
	}
	return &t
}

func invalidData(detail string) *core.APIError {
	return core.Err(http.StatusServiceUnavailable, "invalid_data", detail)
}

func cargar(s *core.Settings, patron string, requeridas []string, que string) (*Loaded, *core.APIError) {
	path := LatestFile(DataDir(s), patron)
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
	return &Loaded{path, df, TimestampDe(path, s.Loc)}, nil
}

// CargarRanking lee el último ranking.
func CargarRanking(s *core.Settings) (*Loaded, *core.APIError) {
	return cargar(s, PatronRanking, ColumnasRequeridasRanking, "un ranking")
}

// CargarIndicadores lee el último CSV de indicadores.
func CargarIndicadores(s *core.Settings) (*Loaded, *core.APIError) {
	return cargar(s, PatronIndicadores, ColumnasIdentidad, "indicadores")
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

// Guardar escribe <prefijo>_<timestamp>.xlsx y .csv (el CSV al final: es el que lee la API) y
// devuelve la ruta del CSV.
func Guardar(df *quant.Frame, dir, prefijo, timestamp string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := filepath.Join(dir, prefijo+"_"+timestamp)
	if err := escribirAtomico(base+".xlsx", func(f *os.File) error { return writeXLSX(df, f) }); err != nil {
		return "", err
	}
	err := escribirAtomico(base+".csv", func(f *os.File) error {
		w := bufio.NewWriter(f)
		if err := df.WriteCSV(w); err != nil {
			return err
		}
		return w.Flush()
	})
	return base + ".csv", err
}

func writeXLSX(df *quant.Frame, f *os.File) error {
	x := excelize.NewFile()
	defer func() { _ = x.Close() }()
	const sheet = "Sheet1"
	for j, c := range df.Cols {
		cell, _ := excelize.CoordinatesToCellName(j+1, 1)
		if err := x.SetCellValue(sheet, cell, c.Name); err != nil {
			return err
		}
	}
	for i := 0; i < df.N; i++ {
		for j, c := range df.Cols {
			v := c.Value(i)
			if v == nil {
				continue
			}
			cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
			if err := x.SetCellValue(sheet, cell, v); err != nil {
				return err
			}
		}
	}
	return x.Write(f)
}
