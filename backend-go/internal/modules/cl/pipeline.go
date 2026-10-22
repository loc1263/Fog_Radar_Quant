package cl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fogradar/internal/core"
	"fogradar/internal/jobs"
	"fogradar/internal/quant"
)

func init() {
	jobs.Register(string(key), jobs.Pipeline{
		Run: Run,
		Options: []jobs.Option{
			{Flag: "--solo-scoring", Help: "Re-puntúa el último CSV de indicadores sin descargar de Yahoo."},
		},
	})
}

// Run es el pipeline batch de Chile: extractor (Yahoo) → scoring → CSV.
//
//	fogradar job cl_stocks                 # completo
//	fogradar job cl_stocks --solo-scoring  # re-puntúa el último CSV de indicadores
//
// Nunca se ejecuta dentro de un request web. Devuelve un resumen para el registro de jobs.
func Run(ctx context.Context, env *jobs.Env, opts map[string]any) (string, error) {
	s := env.S
	inicio := time.Now()
	dir, err := core.NewRun(s, key, inicio)
	if err != nil {
		return "", err
	}
	terminada := false
	defer func() { // una corrida que falla no deja su carpeta a medias
		if !terminada {
			_ = os.RemoveAll(dir)
		}
	}()
	var resumen []string

	var indicadores *quant.Frame
	if b, _ := opts["solo_scoring"].(bool); b {
		fuente, e := CargarIndicadores(s)
		if e != nil {
			return "", e
		}
		indicadores = fuente.DF
		// La corrida nueva lleva su propia copia de los indicadores: así no depende de la corrida de origen.
		if _, err := Guardar(indicadores, dir, ArchivoIndicadores); err != nil {
			return "", err
		}
		resumen = append(resumen, "solo scoring de los indicadores de "+filepath.Base(filepath.Dir(fuente.Path)))
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
		ruta, err := Guardar(indicadores, dir, ArchivoIndicadores)
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
	ruta, err := Guardar(ranking, dir, ArchivoRanking)
	if err != nil {
		return "", err
	}
	env.Log.Info("ranking", "archivo", filepath.Base(dir)+"/"+filepath.Base(ruta))

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
	if err := core.FinishRun(dir, key, inicio, time.Now()); err != nil {
		return "", err
	}
	terminada = true
	return strings.Join(resumen, " · "), nil
}
