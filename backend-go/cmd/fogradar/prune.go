package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"fogradar/internal/core"
	"fogradar/internal/retencion"
)

// runPrune implementa `fogradar prune [--dias N] [--dry-run]`: la retención que los jobs aplican solos,
// a pedido y sobre todos los módulos.
func runPrune(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("prune", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dias := fs.Int("dias", -1, "días a conservar (por defecto RETENCION_DIAS, 7)")
	simular := fs.Bool("dry-run", false, "solo lista lo que borraría")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "fogradar prune:", err)
		return exitUso
	}
	s, err := core.LoadSettings()
	if err != nil {
		fmt.Fprintln(stderr, "fogradar prune:", err)
		return exitUso
	}
	n := s.RetencionDias
	if *dias >= 0 {
		n = *dias
	}
	if n <= 0 {
		fmt.Fprintln(stdout, "retención desactivada (RETENCION_DIAS=0): no se borra nada")
		return exitOK
	}
	res, err := retencion.Podar(s, n, time.Now(), *simular)
	for _, p := range res.Eliminados {
		fmt.Fprintln(stdout, p)
	}
	verbo := "eliminados"
	if *simular {
		verbo = "a eliminar (simulación)"
	}
	fmt.Fprintf(stdout, "retención %d días: %d %s, %.1f MB\n", n, len(res.Eliminados), verbo, float64(res.Bytes)/(1<<20))
	if err != nil {
		fmt.Fprintln(stderr, "fogradar prune:", err)
		return exitFalla
	}
	return exitOK
}
