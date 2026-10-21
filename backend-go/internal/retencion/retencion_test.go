package retencion

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fogradar/internal/core"
)

func tocar(t *testing.T, base string, rutas ...string) {
	t.Helper()
	for _, r := range rutas {
		p := filepath.Join(base, filepath.FromSlash(r))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func existe(base, r string) bool {
	_, err := os.Stat(filepath.Join(base, filepath.FromSlash(r)))
	return err == nil
}

func settings(t *testing.T) *core.Settings {
	t.Helper()
	return &core.Settings{DataDir: t.TempDir(), Loc: time.UTC}
}

func TestPodaConservaVentanaYLoMasReciente(t *testing.T) {
	s := settings(t)
	d := s.DataDir
	ahora := time.Date(2026, 10, 21, 18, 0, 0, 0, time.UTC)
	tocar(t, d,
		// Chile: la última de cada familia se conserva aunque sea vieja
		"cl_stocks/indicadores_acciones_20260901_100000.csv", "cl_stocks/indicadores_acciones_20261015_100000.csv", "cl_stocks/indicadores_acciones_20261021_100000.csv",
		"cl_stocks/ranking_acciones_20260801_100000.csv",
		// EEUU: carpetas con id ISO
		"us_stocks/runs/2026-09-18T18-40-04Z/result.json", "us_stocks/runs/2026-10-16T18-40-04Z/result.json", "us_stocks/runs/2026-10-21T18-40-04Z/result.json",
		// Tendencia
		"tendencia/output/20260921_01/dashboard_data.json", "tendencia/output/20261015_01/dashboard_data.json", "tendencia/output/20261021_02/dashboard_data.json",
		"tendencia/macro/macro_data_20260811.csv", "tendencia/macro/macro_data_20261020.csv", "tendencia/macro/fomc_latest_statement.txt",
		"tendencia/universe/universe_20260811.json", "tendencia/universe/universe_20261017.json", "tendencia/universe/_debug_raw_quote_asia.json",
		// ETF
		"etf/hist/etf_top15_recomendados_20261001.csv", "etf/hist/etf_top15_recomendados_20261020.csv",
		"etf/hist/etf_perf_data_20260710.csv", "etf/hist/etf_perf_data_20260801.csv", "etf/hist/etf_perf_data_20261020.csv",
		"etf/screener/etf_choice_20260901_100000.csv", "etf/screener/etf_choice_20261021_100000.csv", "etf/screener/info_cache.json",
		"etf/screener/data_old/List_All_ETF_valid_20260717_222944.csv",
		"etf/current/etf_top15_recomendados.csv",
	)

	r, err := Podar(s, 7, ahora, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Eliminados) == 0 || r.Bytes == 0 {
		t.Fatalf("no eliminó nada: %+v", r)
	}
	quedan := []string{
		"cl_stocks/indicadores_acciones_20261015_100000.csv", "cl_stocks/indicadores_acciones_20261021_100000.csv", "cl_stocks/ranking_acciones_20260801_100000.csv",
		"us_stocks/runs/2026-10-16T18-40-04Z/result.json", "us_stocks/runs/2026-10-21T18-40-04Z/result.json",
		"tendencia/output/20261015_01/dashboard_data.json", "tendencia/output/20261021_02/dashboard_data.json",
		"tendencia/macro/macro_data_20261020.csv", "tendencia/macro/fomc_latest_statement.txt",
		"tendencia/universe/universe_20261017.json", "tendencia/universe/_debug_raw_quote_asia.json",
		"etf/hist/etf_top15_recomendados_20261020.csv",
		"etf/hist/etf_perf_data_20260710.csv", "etf/hist/etf_perf_data_20261020.csv", // el más antiguo es el ancla de la consistencia
		"etf/screener/etf_choice_20261021_100000.csv", "etf/screener/info_cache.json", "etf/current/etf_top15_recomendados.csv",
	}
	for _, q := range quedan {
		if !existe(d, q) {
			t.Errorf("se borró %s y debía quedar", q)
		}
	}
	borrados := []string{
		"cl_stocks/indicadores_acciones_20260901_100000.csv",
		"us_stocks/runs/2026-09-18T18-40-04Z",
		"tendencia/output/20260921_01", "tendencia/macro/macro_data_20260811.csv", "tendencia/universe/universe_20260811.json",
		"etf/hist/etf_top15_recomendados_20261001.csv", "etf/hist/etf_perf_data_20260801.csv",
		"etf/screener/etf_choice_20260901_100000.csv", "etf/screener/data_old",
	}
	for _, b := range borrados {
		if existe(d, b) {
			t.Errorf("%s debía borrarse", b)
		}
	}
}

func TestSimularNoBorraYDiasCeroDesactiva(t *testing.T) {
	s := settings(t)
	tocar(t, s.DataDir, "tendencia/output/20260101_01/a.json", "tendencia/output/20261021_01/a.json")
	ahora := time.Date(2026, 10, 21, 18, 0, 0, 0, time.UTC)
	r, err := Podar(s, 7, ahora, true)
	if err != nil || len(r.Eliminados) != 1 || !existe(s.DataDir, "tendencia/output/20260101_01/a.json") {
		t.Fatalf("simulación: %+v %v", r, err)
	}
	if r, _ := Podar(s, 0, ahora, false); len(r.Eliminados) != 0 || !existe(s.DataDir, "tendencia/output/20260101_01/a.json") {
		t.Fatalf("dias=0 debe desactivar: %+v", r)
	}
}
