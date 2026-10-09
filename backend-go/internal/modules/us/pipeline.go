package us

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"fogradar/internal/jobs"
)

func init() {
	jobs.Register(string(key), jobs.Pipeline{Run: Run, Options: []jobs.Option{jobs.Backfill}})
}

// Run es el pipeline batch del screener EEUU (7 pasos de RunBatch).
//
//	fogradar job us_stocks              # ~6 min (500 tickers)
//	fogradar job us_stocks --backfill   # solo reconstruye history.db
//
// Nunca se ejecuta dentro de un request web.
func Run(ctx context.Context, env *jobs.Env, opts map[string]any) (string, error) {
	s := env.S
	if b, _ := opts["backfill"].(bool); b {
		return Backfill(s, func(f string, a ...any) { env.Log.Warn(fmt.Sprintf(f, a...)) })
	}
	runDir, err := RunBatch(ctx, env, BatchOpts{OutputRoot: RunsDir(s)})
	if err != nil {
		return "", err
	}
	var result RunResponse
	if e := ReadRunFile(runDir, "result.json", &result); e != nil {
		return "", e
	}
	if result.Status == "error" {
		msg := ""
		if result.Error != nil {
			msg = *result.Error
		}
		return "", fmt.Errorf("La corrida %s falló: %s", filepath.Base(runDir), msg)
	}
	// El índice histórico es derivado: si falla se avisa, pero la corrida no falla (se reconstruye con --backfill).
	if err := GuardarCorrida(s, runDir); err != nil {
		env.Log.Warn("la corrida no se guardó en history.db; se recupera con: fogradar job us_stocks --backfill", "error", err)
	}
	var top []string
	for i := 0; i < len(result.Rows) && i < 3; i++ {
		top = append(top, result.Rows[i].Ticker)
	}
	t := "—"
	if len(top) > 0 {
		t = strings.Join(top, ", ")
	}
	return fmt.Sprintf("%s · %d candidatos de %d · top 3: %s", filepath.Base(runDir), result.CandidatesTotal, result.UniverseTotal, t), nil
}
