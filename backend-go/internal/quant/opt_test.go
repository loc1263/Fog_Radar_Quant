package quant

import (
	"errors"
	"math"
	"testing"

	"gonum.org/v1/gonum/mat"
)

func dense(rows [][]float64) *mat.Dense {
	n := len(rows)
	d := mat.NewDense(n, n, nil)
	for i := range rows {
		for j := range rows[i] {
			d.Set(i, j, rows[i][j])
		}
	}
	return d
}

func TestMaxSharpeSinExceso(t *testing.T) {
	cov := dense([][]float64{{0.04, 0}, {0, 0.09}})
	if _, err := MaxSharpe([]float64{0.01, 0.02}, cov, 0.04, 1, 0); !errors.Is(err, ErrSinExcesoPositivo) {
		t.Fatalf("esperaba ErrSinExcesoPositivo: %v", err)
	}
}

func TestPctChange(t *testing.T) {
	got := PctChange([]float64{100, 110, 99})
	if len(got) != 2 || math.Abs(got[0]-0.1) > 1e-15 || math.Abs(got[1]+0.1) > 1e-15 {
		t.Fatal(got)
	}
	if PctChange([]float64{1}) != nil {
		t.Fatal("un solo precio no da retornos")
	}
}
