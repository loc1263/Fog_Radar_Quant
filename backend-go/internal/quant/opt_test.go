package quant

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"sort"
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

func TestCovCorrComoPandas(t *testing.T) {
	for ci, c := range loadOpt(t) {
		cov, corr := Cov(c.Returns), Corr(c.Returns)
		for i := range c.CovSample {
			for j := range c.CovSample {
				if math.Abs(cov.At(i, j)-c.CovSample[i][j]) > 1e-15+1e-12*math.Abs(c.CovSample[i][j]) {
					t.Fatalf("caso %d cov[%d][%d]: %v vs %v", ci, i, j, cov.At(i, j), c.CovSample[i][j])
				}
				if math.Abs(corr.At(i, j)-c.Corr[i][j]) > 1e-12 {
					t.Fatalf("caso %d corr[%d][%d]: %v vs %v", ci, i, j, corr.At(i, j), c.Corr[i][j])
				}
			}
		}
	}
}

func TestMaxSharpeIgualASLSQP(t *testing.T) {
	for ci, c := range loadOpt(t) {
		w, err := MaxSharpe(c.MeanAnn, dense(c.CovAnn), c.Rf, math.Max(c.Cap, 1/float64(len(c.MeanAnn))), 0)
		if err != nil {
			t.Fatalf("caso %d: %v", ci, err)
		}
		var sum, maxd float64
		for i := range w {
			sum += w[i]
			maxd = math.Max(maxd, math.Abs(w[i]-c.W[i]))
			if w[i] < -1e-9 || w[i] > c.Cap+1e-7 {
				t.Errorf("caso %d: peso fuera de cotas: %v", ci, w[i])
			}
		}
		if math.Abs(sum-1) > 1e-8 {
			t.Errorf("caso %d: los pesos suman %v", ci, sum)
		}
		// SLSQP termina con ftol=1e-10; ambos son óptimos del mismo problema estrictamente convexo.
		if maxd > 2e-5 {
			t.Errorf("caso %d: max|Δw| = %.2e", ci, maxd)
		}
		t.Logf("caso %d (n=%d, tope %.2f): max|Δw| = %.2e", ci, len(w), c.Cap, maxd)
	}
}

func TestMaxSharpeSinExceso(t *testing.T) {
	cov := dense([][]float64{{0.04, 0}, {0, 0.09}})
	if _, err := MaxSharpe([]float64{0.01, 0.02}, cov, 0.04, 1, 0); !errors.Is(err, ErrSinExcesoPositivo) {
		t.Fatalf("esperaba ErrSinExcesoPositivo: %v", err)
	}
}

func TestLinkageYFclusterComoScipy(t *testing.T) {
	for ci, c := range loadOpt(t) {
		n := len(c.Corr)
		dist := make([][]float64, n)
		for i := range dist {
			dist[i] = make([]float64, n)
			for j := range dist[i] {
				cv := math.Max(-1, math.Min(1, c.Corr[i][j]))
				if i != j {
					dist[i][j] = math.Sqrt(0.5 * (1 - cv))
				}
			}
		}
		merges := AverageLinkage(dist)
		hs := make([]float64, len(merges))
		for i, m := range merges {
			hs[i] = m.Dist
		}
		want := append([]float64(nil), c.LinkHeight...)
		sort.Float64s(hs)
		sort.Float64s(want)
		for i := range hs {
			if math.Abs(hs[i]-want[i]) > 1e-9 {
				t.Fatalf("caso %d: altura %d: %v vs %v", ci, i, hs[i], want[i])
			}
		}
		for ks, wl := range c.Labels {
			var k int
			for _, ch := range ks {
				k = k*10 + int(ch-'0')
			}
			got := FclusterMaxclust(merges, n, k)
			if !mismaParticion(got, wl) {
				t.Errorf("caso %d k=%d: particiones distintas\n go: %v\n py: %v", ci, k, got, wl)
			}
		}
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
