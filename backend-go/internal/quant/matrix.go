package quant

import (
	"math"

	"gonum.org/v1/gonum/mat"
)

// PctChange son los retornos simples de una serie de precios (p[t]/p[t-1] − 1), de largo len(p)-1.
func PctChange(p []float64) []float64 {
	if len(p) < 2 {
		return nil
	}
	out := make([]float64, len(p)-1)
	for i := 1; i < len(p); i++ {
		out[i-1] = p[i]/p[i-1] - 1
	}
	return out
}

// MeanOf es la media aritmética.
func MeanOf(a []float64) float64 {
	if len(a) == 0 {
		return NaN
	}
	return pairwiseSum(a) / float64(len(a))
}

// Cov es la matriz de covarianza muestral (ddof=1) de las columnas (series de igual largo).
func Cov(cols [][]float64) *mat.Dense {
	k := len(cols)
	out := mat.NewDense(k, k, nil)
	if k == 0 {
		return out
	}
	t := len(cols[0])
	means := make([]float64, k)
	for i := range cols {
		means[i] = MeanOf(cols[i])
	}
	prod := make([]float64, t)
	for i := 0; i < k; i++ {
		for j := i; j < k; j++ {
			for r := 0; r < t; r++ {
				prod[r] = (cols[i][r] - means[i]) * (cols[j][r] - means[j])
			}
			v := pairwiseSum(prod) / float64(t-1)
			out.Set(i, j, v)
			out.Set(j, i, v)
		}
	}
	return out
}

// Corr es la matriz de correlación de las columnas; NaN (varianza 0) se reporta como NaN.
func Corr(cols [][]float64) *mat.Dense {
	c := Cov(cols)
	k, _ := c.Dims()
	out := mat.NewDense(k, k, nil)
	for i := 0; i < k; i++ {
		for j := 0; j < k; j++ {
			if i == j {
				out.Set(i, j, 1)
				continue
			}
			out.Set(i, j, c.At(i, j)/math.Sqrt(c.At(i, i)*c.At(j, j)))
		}
	}
	return out
}
