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
//	output/<YYYYMMDD>_<NN>/  una carpeta por corrida (dashboard_data.json, CSV/JSON de Markowitz, tendencia y momentum)
const (
	key = core.ModuleKey("tendencia")
	// RunDirPattern es el nombre válido de una carpeta de corrida.
	RunDirPattern = `^\d{8}_\d{2}$`
	// DashboardFile es el JSON de una corrida.
	DashboardFile = "dashboard_data.json"
	sinCorridas   = "Todavía no hay corridas de Tendencia. Ejecuta: fogradar job tendencia"
)

var (
	runDirRe = regexp.MustCompile(RunDirPattern)
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
