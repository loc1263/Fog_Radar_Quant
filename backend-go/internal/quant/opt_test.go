package quant

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"testing"

	"gonum.org/v1/gonum/mat"
)

type optCase struct {
	Returns    [][]float64      `json:"returns"`
	CovSample  [][]float64      `json:"cov_sample"`
	Corr       [][]float64      `json:"corr"`
	MeanAnn    []float64        `json:"mean_ann"`
	CovAnn     [][]float64      `json:"cov_ann"`
	Rf         float64          `json:"rf"`
	Cap        float64          `json:"cap"`
	W          []float64        `json:"w"`
	Labels     map[string][]int `json:"labels"`
	LinkHeight []float64        `json:"link_heights"`
}

func loadOpt(t *testing.T) []optCase {
	t.Helper()
	b, err := os.ReadFile("testdata/opt.json")
	if err != nil {
		t.Fatal(err)
	}
	var cs []optCase
	if err := json.Unmarshal(b, &cs); err != nil {
		t.Fatal(err)
	}
	return cs
}

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

// mismaParticion: dos etiquetados son iguales si agrupan los mismos elementos (sin importar los números).
func mismaParticion(a, b []int) bool {
	ab, ba := map[int]int{}, map[int]int{}
	for i := range a {
		if v, ok := ab[a[i]]; ok && v != b[i] {
			return false
		}
		if v, ok := ba[b[i]]; ok && v != a[i] {
			return false
		}
		ab[a[i]], ba[b[i]] = b[i], a[i]
	}
	return true
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
