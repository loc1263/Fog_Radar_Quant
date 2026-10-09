package cl

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fogradar/internal/jobs"
	"fogradar/internal/quant"
)

func init() {
	jobs.Register(string(key), jobs.Pipeline{
		Run: Run,
		Options: []jobs.Option{
			{Flag: "--solo-scoring", Help: "Re-puntúa el último CSV de indicadores sin descargar de Yahoo."},
			jobs.Backfill,
		},
	})
}

// Run es el pipeline batch de Chile: extractor (Yahoo) → scoring → CSV/XLSX → history.db.
//
//	fogradar job cl_stocks                 # completo
//	fogradar job cl_stocks --solo-scoring  # re-puntúa el último CSV de indicadores
//	fogradar job cl_stocks --backfill      # solo reconstruye history.db
//
// Nunca se ejecuta dentro de un request web. Devuelve un resumen para el registro de jobs.
func Run(ctx context.Context, env *jobs.Env, opts map[string]any) (string, error) {
	s := env.S
	if b, _ := opts["backfill"].(bool); b {
		return Backfill(s, func(f string, a ...any) { env.Log.Warn(fmt.Sprintf(f, a...)) })
	}
	dir := DataDir(s)
	timestamp := time.Now().In(s.Loc).Format("20060102_150405")
	var resumen []string

	var indicadores *quant.Frame
	if b, _ := opts["solo_scoring"].(bool); b {
		fuente, e := CargarIndicadores(s)
		if e != nil {
			return "", e
		}
		indicadores = fuente.DF
		// El ranking conserva el timestamp de sus datos: re-puntuar no los hace «más nuevos».
		timestamp = strings.TrimSuffix(strings.TrimPrefix(filepath.Base(fuente.Path), "indicadores_acciones_"), ".csv")
		resumen = append(resumen, "solo scoring de "+filepath.Base(fuente.Path))
	} else {
		tickers, err := CargarTickers()
		if err != nil {
			return "", err
		}
		var errores []string
		indicadores, errores = ExtraerIndicadores(ctx, env.Yahoo, tickers, ExtraerOpts{Concurrencia: env.S.YahooConcurrencia, Log: env.Log})
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if indicadores.N == 0 {
			return "", errors.New("Yahoo Finance no devolvió datos para ningún ticker.")
		}
		ruta, err := Guardar(indicadores, dir, "indicadores_acciones", timestamp)
		if err != nil {
			return "", err
		}
		env.Log.Info("indicadores", "archivo", filepath.Base(ruta))
		resumen = append(resumen, fmt.Sprintf("%d/%d tickers", indicadores.N, len(tickers)))
		if len(errores) > 0 {
			resumen = append(resumen, "sin datos: "+strings.Join(errores, ", "))
		}
	}

	ranking, err := ConstruirRanking(indicadores, nil)
	if err != nil {
		return "", err
	}
	ruta, err := Guardar(ranking, dir, "ranking_acciones", timestamp)
	if err != nil {
		return "", err
	}
	env.Log.Info("ranking", "archivo", filepath.Base(ruta))
	// El índice histórico es derivado: si falla se avisa, pero la corrida no falla (se reconstruye con --backfill).
	if err := GuardarCorrida(s, ruta); err != nil {
		env.Log.Warn("la corrida no se guardó en history.db; se recupera con: fogradar job cl_stocks --backfill", "error", err)
	}

	sym := ranking.Col("Símbolo")
	var top []string
	for i := 0; i < ranking.N && i < 3; i++ {
		top = append(top, sym.Cell(i))
	}
	resumen = append(resumen, "top 3: "+strings.Join(top, ", "))
	grupos := GruposDuplicados(ranking)
	ids := make([]int, 0, len(grupos))
	for g := range grupos {
		ids = append(ids, g)
	}
	sort.Ints(ids)
	for _, g := range ids {
		env.Log.Warn("grupo financiero duplicado (riesgo de doble conteo)", "grupo", g, "simbolos", strings.Join(grupos[g], ", "))
	}
	return strings.Join(resumen, " · "), nil
}
