package cl

import (
	"strings"
	"testing"

	"fogradar/internal/quant"
)

func TestFaltanColumnasDeIdentidad(t *testing.T) {
	f := quant.NewFrame(1)
	f.SetFloats("Símbolo", []float64{1})
	if _, err := ConstruirRanking(f, nil); err == nil || !strings.Contains(err.Error(), "Nombre corto") {
		t.Fatal(err)
	}
}

func TestDuplicadosFinancieros(t *testing.T) {
	f := quant.NewFrame(4)
	f.SetFloats("Revenue (ttm)", []float64{100, 100.05, 500, quant.NaN})
	f.SetFloats("EBITDA", []float64{10, 10, 50, quant.NaN})
	g := DetectarDuplicados(f)
	if g[0] != 1 || g[1] != 1 || g[2] != 0 || g[3] != 0 {
		t.Fatalf("%v", g)
	}
}
