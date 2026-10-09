package quant

import (
	"errors"
	"math"

	"gonum.org/v1/gonum/mat"
)

// Equivalentes de PyPortfolioOpt (expected_returns, risk_models, EfficientFrontier) que usa el módulo Tendencia.

// PctChangeNaN son los retornos simples p[t]/p[t−1] − 1 de una serie con huecos (fill_method=None): el primero y
// los que tocan un NaN quedan NaN. Mismo largo que la entrada.
func PctChangeNaN(p []float64) []float64 {
	out := make([]float64, len(p))
	for i := range p {
		if i == 0 || math.IsNaN(p[i]) || math.IsNaN(p[i-1]) {
			out[i] = NaN
		} else {
			out[i] = p[i]/p[i-1] - 1
		}
	}
	return out
}

// ReturnsFromPrices calcula los retornos de cada columna y descarta las filas donde TODAS son NaN
// (returns_from_prices: pct_change(fill_method=None).dropna(how="all")).
func ReturnsFromPrices(prices [][]float64) [][]float64 {
	if len(prices) == 0 {
		return nil
	}
	r := make([][]float64, len(prices))
	for j := range prices {
		r[j] = PctChangeNaN(prices[j])
	}
	n := len(r[0])
	keep := make([]bool, n)
	for i := 0; i < n; i++ {
		for j := range r {
			if !math.IsNaN(r[j][i]) {
				keep[i] = true
				break
			}
		}
	}
	out := make([][]float64, len(r))
	for j := range r {
		for i := 0; i < n; i++ {
			if keep[i] {
				out[j] = append(out[j], r[j][i])
			}
		}
	}
	return out
}

// MeanHistoricalReturn: retorno anualizado compuesto (CAGR) de cada columna: Π(1+r)^(freq/n) − 1 sobre los r presentes.
func MeanHistoricalReturn(returns [][]float64, freq float64) []float64 {
	out := make([]float64, len(returns))
	for j, col := range returns {
		prod, n := 1.0, 0
		for _, r := range col {
			if !math.IsNaN(r) {
				prod *= 1 + r
				n++
			}
		}
		out[j] = math.Pow(prod, freq/float64(n)) - 1
	}
	return out
}

// pairCov replica DataFrame.cov() de pandas para un par: observaciones completas del par, algoritmo en línea.
func pairCov(x, y []float64) float64 {
	var nobs float64
	var meanx, meany, covxy float64
	for i := range x {
		if math.IsNaN(x[i]) || math.IsNaN(y[i]) {
			continue
		}
		nobs++
		prevy := meany
		meanx += 1 / nobs * (x[i] - meanx)
		meany += 1 / nobs * (y[i] - meany)
		covxy += (x[i] - meanx) * (y[i] - prevy)
	}
	if nobs < 2 {
		return NaN
	}
	return covxy / (nobs - 1)
}

// CovPairwise es la covarianza muestral con observaciones completas por par (como pandas), multiplicada por freq.
func CovPairwise(returns [][]float64, freq float64) *mat.Dense {
	k := len(returns)
	out := mat.NewDense(k, k, nil)
	for i := 0; i < k; i++ {
		for j := i; j < k; j++ {
			v := pairCov(returns[i], returns[j]) * freq
			out.Set(i, j, v)
			out.Set(j, i, v)
		}
	}
	return out
}

func isPSD(m *mat.Dense) bool {
	n, _ := m.Dims()
	a := mat.NewSymDense(n, nil)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			v := m.At(i, j)
			if i == j {
				v += 1e-16
			}
			a.SetSym(i, j, v)
		}
	}
	var ch mat.Cholesky
	return ch.Factorize(a)
}

// FixNonPositiveSemidefinite deja en cero los autovalores negativos y reconstruye la matriz (método «spectral»).
func FixNonPositiveSemidefinite(m *mat.Dense) *mat.Dense {
	if isPSD(m) {
		return m
	}
	n, _ := m.Dims()
	s := mat.NewSymDense(n, nil)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			s.SetSym(i, j, m.At(i, j))
		}
	}
	var es mat.EigenSym
	if !es.Factorize(s, true) {
		return m
	}
	vals := es.Values(nil)
	var v mat.Dense
	es.VectorsTo(&v)
	d := mat.NewDiagDense(n, nil)
	for i, q := range vals {
		if q > 0 {
			d.SetDiag(i, q)
		}
	}
	var tmp, fixed mat.Dense
	tmp.Mul(&v, d)
	fixed.Mul(&tmp, v.T())
	return &fixed
}

// SampleCov: sample_cov de PyPortfolioOpt (cov pairwise · freq, corregida a semidefinida positiva).
func SampleCov(returns [][]float64, freq float64) *mat.Dense {
	return FixNonPositiveSemidefinite(CovPairwise(returns, freq))
}

// ErrMinVolInfactible: no hay pesos long-only con ese tope que sumen 1.
var ErrMinVolInfactible = errors.New("no hay pesos factibles que sumen 1 con el tope dado")

// MinVolatility minimiza w'Σw + γ‖w‖² con Σw = 1 y 0 ≤ w ≤ ub (EfficientFrontier.min_volatility).
func MinVolatility(cov *mat.Dense, ub, gamma float64) ([]float64, error) {
	n, _ := cov.Dims()
	if ub*float64(n) < 1-1e-12 {
		return nil, ErrMinVolInfactible
	}
	P := mat.NewDense(n, n, nil)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			v := 2 * cov.At(i, j)
			if i == j {
				v += 2 * gamma
			}
			P.Set(i, j, v)
		}
	}
	A := mat.NewDense(n+1, n, nil)
	l, u := make([]float64, n+1), make([]float64, n+1)
	for i := 0; i < n; i++ {
		A.Set(0, i, 1)
		A.Set(i+1, i, 1)
		l[i+1], u[i+1] = 0, ub
	}
	l[0], u[0] = 1, 1
	x, _, err := QP(P, A, make([]float64, n), l, u, 1e-10, 400_000)
	if err != nil {
		return nil, err
	}
	for i := range x {
		x[i] = math.Round(x[i]*1e16) / 1e16 // .round(16)
	}
	return x, nil
}

// CleanWeights pone en cero los pesos con |w| < cutoff y redondea (np.round) a rounding decimales; no renormaliza.
func CleanWeights(w []float64, cutoff float64, rounding int) []float64 {
	out := make([]float64, len(w))
	for i, v := range w {
		if math.Abs(v) < cutoff {
			v = 0
		}
		out[i] = Round(v, rounding)
		if out[i] == 0 {
			out[i] = 0 // quita el cero con signo
		}
	}
	return out
}

// PortfolioPerformance devuelve retorno esperado, volatilidad y Sharpe de los pesos w.
func PortfolioPerformance(w, mu []float64, cov *mat.Dense, rf float64) (ret, vol, sharpe float64) {
	for i := range w {
		ret += w[i] * mu[i]
	}
	n := len(w)
	v := 0.0
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			v += w[i] * cov.At(i, j) * w[j]
		}
	}
	vol = math.Sqrt(v)
	return ret, vol, (ret - rf) / vol
}
