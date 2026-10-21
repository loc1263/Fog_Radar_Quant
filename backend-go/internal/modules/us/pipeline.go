package us

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fogradar/internal/core"
	"fogradar/internal/jobs"
)

func init() {
	jobs.Register(string(key), jobs.Pipeline{Run: Run})
}

// Run es el pipeline batch del screener EEUU (7 pasos de RunBatch).
//
//	fogradar job us_stocks              # ~6 min (500 tickers)
//
// Nunca se ejecuta dentro de un request web.
func Run(ctx context.Context, env *jobs.Env, opts map[string]any) (string, error) {
	inicio := time.Now()
	runDir, err := core.NewRun(env.S, key, inicio)
	if err != nil {
		return "", err
	}
	if _, err := RunBatch(ctx, env, runDir, BatchOpts{}); err != nil {
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
		return "", fmt.Errorf("La corrida %s falló: %s", filepath.Base(runDir), msg) // sin run.json: nadie la lee
	}
	if err := core.FinishRun(runDir, key, inicio, time.Now()); err != nil {
		return "", err
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
