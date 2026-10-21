package etf

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
			{Flag: "--refrescar-info", Help: "Ignora la caché de info de ETFs (nombre, AUM, categoría…) y la pide de nuevo a Yahoo."},
		},
	})
}

func fechaTagDe(origen string, s *core.Settings) string {
	if m := fechaDigits.FindString(filepath.Base(origen)); m != "" {
		return m
	}
	return time.Now().In(s.Loc).Format("20060102")
}

// copyDir copia el contenido de src (si existe) dentro de dst.
func copyDir(src, dst string) error {
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return nil
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		return copyFile(p, filepath.Join(dst, rel))
	})
}

// analizar corre los análisis dentro de runDir: copia el CSV de origen con su nombre final y el historial de corridas
// de la corrida anterior, y deja ahí los outputs. El ancla de la consistencia del momentum se guarda la primera vez.
func analizar(env *jobs.Env, runDir, origenCSV, previa string) (string, error) {
	s := env.S
	tag := fechaTagDe(origenCSV, s)
	csv := filepath.Join(runDir, "etf_perf_data_"+tag+".csv")
	if err := copyFile(origenCSV, csv); err != nil {
		return "", err
	}
	if previa != "" {
		if h := filepath.Join(previa, HistorialFile); fileOK(h) {
			if err := copyFile(h, filepath.Join(runDir, HistorialFile)); err != nil {
				return "", err
			}
		}
	}
	warn := func(f string, a ...any) { env.Log.Warn(fmt.Sprintf(f, a...)) }
	env.Log.Info("corriendo el análisis", "corrida", filepath.Base(runDir))
	res, err := CorrerAnalisis(runDir, BaselinePath(s), warn)
	if err != nil {
		return "", err
	}
	if base := BaselinePath(s); !fileOK(base) {
		if err := os.MkdirAll(filepath.Dir(base), 0o755); err == nil {
			if err := copyFile(csv, base); err != nil {
				warn("No se pudo guardar el ancla de consistencia: %v", err)
			} else {
				env.Log.Info("ancla de consistencia guardada", "archivo", BaselineFile)
			}
		}
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

// Run es el pipeline batch de ETFs.
//
//	fogradar job etf                  # screener Yahoo + análisis
//	fogradar job etf --solo-analisis  # re-analiza el CSV vigente (segundos)
//
// 1) screener → runs/<id>/raw/; 2) análisis en runs/<id>/ (run.json se escribe al final: hasta entonces la corrida
// no existe para los lectores). Un --solo-analisis reemplaza a la corrida que re-analiza.
func Run(ctx context.Context, env *jobs.Env, opts map[string]any) (string, error) {
	s := env.S
	inicio := time.Now()
	vigente := CurrentDir(s) // la última terminada, antes de crear la nueva
	runDir, err := core.NewRun(s, key, inicio)
	if err != nil {
		return "", err
	}
	terminada := false
	defer func() { // una corrida que falla no deja su carpeta a medias
		if !terminada {
			_ = os.RemoveAll(runDir)
		}
	}()

	var resumen, origen string
	if b, _ := opts["solo_analisis"].(bool); b {
		if vigente == "" || CSVActual(vigente) == "" {
			return "", core.NoData(SinCorrida)
		}
		origen = CSVActual(vigente)
		if err := copyDir(filepath.Join(vigente, "raw"), filepath.Join(runDir, "raw")); err != nil {
			return "", err
		}
		resumen, err = analizar(env, runDir, origen, vigente)
		if err != nil {
			return "", err
		}
		resumen = "solo análisis · " + resumen
	} else {
		refrescar, _ := opts["refrescar_info"].(bool)
		env.Log.Info("corriendo el screener de ETFs", "refrescar_info", refrescar)
		res, err := ProcesarETFs(ctx, env.Yahoo, ScreenerOpts{OutputDir: filepath.Join(runDir, "raw"), CacheDir: CacheDir(s),
			Concurrencia: s.YahooConcurrencia, Log: env.Log, RefrescarInfo: refrescar})
		if err != nil {
			return "", err
		}
		if res == nil || res.CSV == "" {
			return "", errors.New("El screener terminó sin generar un etf_perf_data_*.csv (ningún ETF validado).")
		}
		env.Log.Info("screener generó", "archivo", filepath.Base(res.CSV), "validos", res.NValidos, "invalidos", res.NInvalidos)
		resumen, err = analizar(env, runDir, res.CSV, vigente)
		if err != nil {
			return "", err
		}
	}
	if err := core.FinishRun(runDir, key, inicio, time.Now()); err != nil {
		return "", err
	}
	terminada = true
	if origen != "" && vigente != "" { // el re-análisis reemplaza a la corrida de origen
		_ = os.RemoveAll(vigente)
	}
	return resumen, nil
}
