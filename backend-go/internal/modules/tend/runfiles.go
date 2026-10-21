package tend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"fogradar/internal/core"
)

// Archivos de cada corrida (data/tendencia/runs/<id>/): dashboard_data.json, CSV/JSON de Markowitz, tendencia y
// momentum, run_manifest.json y processed/ (el dataset de esa corrida).
const (
	key = core.ModuleKey("tendencia")
	// DashboardFile es el JSON de una corrida.
	DashboardFile = "dashboard_data.json"
	sinCorridas   = "Todavía no hay corridas de Tendencia. Ejecuta: fogradar job tendencia"
)

// LatestRun es la corrida terminada más reciente.
func LatestRun(s *core.Settings) (string, *core.APIError) {
	if d := core.LatestRun(s, key); d != "" {
		return d, nil
	}
	return "", core.NoData(sinCorridas)
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
	b, err := os.ReadFile(filepath.Join(latestRunDir(s), "processed", "excluded_tickers.txt"))
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

// latestRunDir es la última corrida terminada ("" si no hay: las rutas que se arman con él no existen).
func latestRunDir(s *core.Settings) string { return core.LatestRun(s, key) }

func isFile(p string) bool { st, err := os.Stat(p); return err == nil && !st.IsDir() }
