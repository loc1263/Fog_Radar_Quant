package tend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"fogradar/internal/core"
)

// Archivos del módulo que lee la API (data/tendencia):
//
//	output/<YYYYMMDD>_<NN>/  una carpeta por corrida (dashboard_data.json, CSV/JSON de Markowitz, tendencia y momentum, PDF)
const (
	key = core.ModuleKey("tendencia")
	// RunDirPattern es el nombre válido de una carpeta de corrida.
	RunDirPattern = `^\d{8}_\d{2}$`
	// DashboardFile es el JSON de una corrida.
	DashboardFile = "dashboard_data.json"
	sinCorridas   = "Todavía no hay corridas de Tendencia. Ejecuta: fogradar job tendencia"
)

var (
	runDirRe       = regexp.MustCompile(RunDirPattern)
	nombreArchivo  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_\-]{0,126}\.[A-Za-z0-9]{1,5}$`)
	tiposDescargar = map[string]string{
		".pdf": "application/pdf", ".csv": "text/csv; charset=utf-8", ".json": "application/json", ".txt": "text/plain; charset=utf-8",
	}
)

// ModuleDir es data/tendencia.
func ModuleDir(s *core.Settings) string { return s.ModuleDataDir(key) }

// OutputDir es data/tendencia/output.
func OutputDir(s *core.Settings) string { return filepath.Join(ModuleDir(s), "output") }

func isFile(p string) bool { st, err := os.Stat(p); return err == nil && !st.IsDir() }

// RunDirs lista las corridas terminadas (con dashboard_data.json), la más reciente primero.
func RunDirs(s *core.Settings) []string {
	ents, err := os.ReadDir(OutputDir(s))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() && runDirRe.MatchString(e.Name()) && isFile(filepath.Join(OutputDir(s), e.Name(), DashboardFile)) {
			out = append(out, filepath.Join(OutputDir(s), e.Name()))
		}
	}
	sort.Slice(out, func(i, j int) bool { return filepath.Base(out[i]) > filepath.Base(out[j]) })
	return out
}

// RunDir devuelve la carpeta de una corrida por nombre (404 si no es válida o no existe).
func RunDir(s *core.Settings, name string) (string, *core.APIError) {
	if !runDirRe.MatchString(name) {
		return "", core.Err(http.StatusNotFound, "not_found", fmt.Sprintf("'%s' no es un nombre de corrida válido (YYYYMMDD_NN).", name))
	}
	d := filepath.Join(OutputDir(s), name)
	if !isFile(filepath.Join(d, DashboardFile)) {
		return "", core.Err(http.StatusNotFound, "not_found", fmt.Sprintf("No existe la corrida '%s'.", name))
	}
	return d, nil
}

// LatestRun es la corrida terminada más reciente.
func LatestRun(s *core.Settings) (string, *core.APIError) {
	d := RunDirs(s)
	if len(d) == 0 {
		return "", core.NoData(sinCorridas)
	}
	return d[0], nil
}

// ReadDashboard lee el dashboard_data.json de una corrida.
func ReadDashboard(d string, v any) *core.APIError {
	b, err := os.ReadFile(filepath.Join(d, DashboardFile))
	if err == nil {
		err = json.Unmarshal(b, v)
	}
	if err != nil {
		return core.Err(http.StatusServiceUnavailable, "invalid_data", fmt.Sprintf("%s/%s está corrupto.", filepath.Base(d), DashboardFile))
	}
	return nil
}

// Archivos lista los archivos descargables de una corrida.
func Archivos(d string) []TdArchivo {
	out := []TdArchivo{}
	ents, _ := os.ReadDir(d)
	for _, e := range ents {
		tipo, ok := tiposDescargar[strings.ToLower(filepath.Ext(e.Name()))]
		if e.IsDir() || !ok || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if info, err := e.Info(); err == nil {
			out = append(out, TdArchivo{Nombre: e.Name(), Tipo: tipo, Bytes: info.Size()})
		}
	}
	return out
}

// Archivo devuelve la ruta y el tipo de un archivo descargable de la corrida; rechaza rutas y extensiones no permitidas.
// Lista blanca (no lista negra): sin separadores, unidades de Windows (C:x) ni flujos alternativos NTFS (a:b); y la
// ruta final debe quedar dentro de la corrida.
func Archivo(d, nombre string) (path, tipo string, e *core.APIError) {
	tipo, ok := tiposDescargar[strings.ToLower(filepath.Ext(nombre))]
	if !nombreArchivo.MatchString(nombre) || !ok {
		return "", "", core.Err(http.StatusNotFound, "not_found", "Archivo no disponible.")
	}
	path = filepath.Join(d, nombre)
	absD, _ := filepath.Abs(d)
	absP, _ := filepath.Abs(path)
	if rel, err := filepath.Rel(absD, absP); err != nil || strings.HasPrefix(rel, "..") || !isFile(path) {
		return "", "", core.Err(http.StatusNotFound, "not_found", fmt.Sprintf("La corrida %s no tiene %s.", filepath.Base(d), nombre))
	}
	return path, tipo, nil
}

// Excluidos: tickers que el último armado del dataset dejó fuera (processed/excluded_tickers.txt: ticker<TAB>nombre<TAB>motivo).
func Excluidos(s *core.Settings) []TdExcluido {
	out := []TdExcluido{}
	b, err := os.ReadFile(filepath.Join(ModuleDir(s), "processed", "excluded_tickers.txt"))
	if err != nil {
		return out
	}
	for _, linea := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		p := strings.Split(linea, "\t")
		if len(p) >= 3 && strings.TrimSpace(p[0]) != "" {
			out = append(out, TdExcluido{Ticker: strings.TrimSpace(p[0]), Nombre: strings.TrimSpace(p[1]), Motivo: strings.TrimSpace(p[2])})
		}
	}
	return out
}
