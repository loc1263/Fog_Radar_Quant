package etf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"fogradar/internal/core"
	"fogradar/internal/jobs"
)

func init() {
	jobs.Register(string(key), jobs.Pipeline{
		Run: Run,
		Options: []jobs.Option{
			{Flag: "--solo-analisis", Help: "Re-analiza el CSV vigente sin descargar de Yahoo."},
			jobs.Backfill,
		},
	})
}

// ArchivoDiasRespaldo: las salidas crudas del screener de más de estos días se mueven a data_old/.
const ArchivoDiasRespaldo = 30

var patronesRespaldoScreener = []string{"etf_perf_data_*.csv", "etf_perf_data_*.xlsx", "etf_choice_*.csv", "etf_choice_*.xlsx",
	"List_All_ETF_valid_*.csv", "incidencias_*.csv"}

var reTimestampCorrida = regexp.MustCompile(`(\d{8}_\d{6})`)

// RespaldarOutputsViejosScreener mueve (no borra) a data_old/ las salidas crudas del screener cuyo timestamp en el nombre
// tenga más de dias días. Una falla puntual se registra y no hace fallar la corrida.
func RespaldarOutputsViejosScreener(dir string, dias int, ahora time.Time, warn func(format string, args ...any)) []string {
	if dias <= 0 {
		return nil
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil
	}
	dataOld := filepath.Join(dir, "data_old")
	if err := os.MkdirAll(dataOld, 0o755); err != nil {
		return nil
	}
	var movidos []string
	for _, pat := range patronesRespaldoScreener {
		m, _ := filepath.Glob(filepath.Join(dir, pat))
		for _, ruta := range m {
			ts := reTimestampCorrida.FindString(filepath.Base(ruta))
			if ts == "" {
				continue
			}
			fecha, err := time.ParseInLocation("20060102_150405", ts, ahora.Location())
			if err != nil {
				continue
			}
			if int(ahora.Sub(fecha).Hours()/24) <= dias {
				continue
			}
			dst := filepath.Join(dataOld, filepath.Base(ruta))
			if err := move(ruta, dst); err != nil {
				warn("No se pudo respaldar %s: %v", filepath.Base(ruta), err)
				continue
			}
			movidos = append(movidos, dst)
		}
	}
	return movidos
}

func fechaTagDe(origen string, s *core.Settings) string {
	if m := fechaDigits.FindString(filepath.Base(origen)); m != "" {
		return m
	}
	return time.Now().In(s.Loc).Format("20060102")
}

func analizarEInstalar(env *jobs.Env, origenCSV string, archivar bool) (string, error) {
	s := env.S
	tag := fechaTagDe(origenCSV, s)
	staging, err := PrepararStaging(s, origenCSV, tag)
	defer func() {
		if staging != "" {
			_ = os.RemoveAll(staging)
		}
	}()
	if err != nil {
		return "", err
	}
	warn := func(f string, a ...any) { env.Log.Warn(fmt.Sprintf(f, a...)) }
	env.Log.Info("corriendo el análisis en staging", "staging", filepath.Base(staging))
	res, err := CorrerAnalisis(staging, HistDir(s), warn)
	if err != nil {
		return "", err
	}
	instalados, err := InstalarDesdeStaging(s, staging, archivar, func(f string, a ...any) { env.Log.Info(fmt.Sprintf(f, a...)) })
	if err != nil {
		return "", err
	}
	env.Log.Info("instalados en current/", "archivos", strings.Join(instalados, ", "))
	// El índice histórico es derivado: si falla se avisa, pero la corrida no falla (se reconstruye con --backfill).
	if _, err := GuardarVigente(s); err != nil {
		env.Log.Warn("la corrida no se guardó en history.db; se recupera con: fogradar job etf --backfill", "error", err)
	}
	var top3 []string
	for i := 0; i < res.Top15.N && i < 3; i++ {
		top3 = append(top3, res.Top15.Col("Ticker").Cell(i))
	}
	partes := []string{
		"corrida " + tag,
		fmt.Sprintf("%d/%d ETFs depurados", res.NUniversoDepurad, res.NUniversoBruto),
		fmt.Sprintf("%d outliers", res.NOutliers),
		"top 3: " + strings.Join(top3, ", "),
	}
	if len(res.Avisos) > 0 {
		partes = append(partes, "sin "+strings.Join(res.Avisos, ", "))
	}
	return strings.Join(partes, " · "), nil
}

// Run es el pipeline batch de ETFs (port de actualizar_datos_semanal.py).
//
//	fogradar job etf                  # screener Yahoo + análisis
//	fogradar job etf --solo-analisis  # re-analiza el CSV vigente (segundos)
//	fogradar job etf --backfill       # solo reconstruye history.db
//
// 1) screener → data/etf/screener/etf_perf_data_<ts>.csv; 2) análisis en una carpeta de staging (current/ no se toca
// todavía); 3) solo si el análisis terminó sin errores: archiva la corrida vigente e instala la nueva (bajo el lock que
// respetan los GET); 4) guarda la corrida en history.db; 5) mueve a screener/data_old/ las salidas crudas de más de 30 días.
func Run(ctx context.Context, env *jobs.Env, opts map[string]any) (string, error) {
	s := env.S
	if b, _ := opts["backfill"].(bool); b {
		return Backfill(s, func(f string, a ...any) { env.Log.Warn(fmt.Sprintf(f, a...)) })
	}
	if b, _ := opts["solo_analisis"].(bool); b {
		unlock, e := BloqueoDatos(s, 60*time.Second)
		if e != nil {
			return "", e
		}
		vigente := CSVActual(CurrentDir(s))
		unlock()
		if vigente == "" {
			return "", core.NoData(SinCorrida)
		}
		r, err := analizarEInstalar(env, vigente, false)
		return "solo análisis · " + r, err
	}

	env.Log.Info("corriendo el screener de ETFs")
	res, err := ProcesarETFs(ctx, env.Yahoo, ScreenerOpts{OutputDir: ScreenerDir(s), Concurrencia: s.YahooConcurrencia, Log: env.Log})
	if err != nil {
		return "", err
	}
	if res == nil || res.CSV == "" {
		return "", errors.New("El screener terminó sin generar un etf_perf_data_*.csv (ningún ETF validado).")
	}
	env.Log.Info("screener generó", "archivo", filepath.Base(res.CSV), "validos", res.NValidos, "invalidos", res.NInvalidos)
	resumen, err := analizarEInstalar(env, res.CSV, true)
	if err != nil {
		return "", err
	}
	if m := RespaldarOutputsViejosScreener(ScreenerDir(s), ArchivoDiasRespaldo, time.Now().In(s.Loc),
		func(f string, a ...any) { env.Log.Warn(fmt.Sprintf(f, a...)) }); len(m) > 0 {
		env.Log.Info("respaldados archivos viejos del screener en data_old/", "n", len(m))
	}
	return resumen, nil
}
