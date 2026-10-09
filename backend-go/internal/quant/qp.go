package quant

import (
	"errors"
	"fmt"
	"math"

	"gonum.org/v1/gonum/mat"
)

// QP resuelve min ½x'Px + q'x  s.t.  l ≤ Ax ≤ u con ADMM (estilo OSQP, ρ adaptativo). Es el solver
// de los optimizadores de portafolio. Devuelve x y las
// iteraciones usadas; error si el problema no converge dentro de maxIter.
func QP(P, A *mat.Dense, q, l, u []float64, tol float64, maxIter int) ([]float64, int, error) {
	n, _ := P.Dims()
	m, _ := A.Dims()
	const sigma, alpha = 1e-6, 1.6
	rho := 0.1
	rhoV := make([]float64, m)
	setRho := func() {
		for i := range rhoV {
			if l[i] == u[i] {
				rhoV[i] = 1e3 * rho
			} else {
				rhoV[i] = rho
			}
		}
	}
	var ch mat.Cholesky
	factor := func() error {
		setRho()
		K := mat.NewSymDense(n, nil)
		for i := 0; i < n; i++ {
			for j := i; j < n; j++ {
				v := P.At(i, j)
				if i == j {
					v += sigma
				}
				for r := 0; r < m; r++ {
					if a, b := A.At(r, i), A.At(r, j); a != 0 && b != 0 {
						v += rhoV[r] * a * b
					}
				}
				K.SetSym(i, j, v)
			}
		}
		if !ch.Factorize(K) {
			return errors.New("matriz KKT no definida positiva")
		}
		return nil
	}
	if err := factor(); err != nil {
		return nil, 0, err
	}
	x, z, y := make([]float64, n), make([]float64, m), make([]float64, m)
	clip := func(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
	Ax := make([]float64, m)
	rhs := make([]float64, n)
	var xt mat.VecDense
	for it := 1; it <= maxIter; it++ {
		for i := range rhs {
			rhs[i] = sigma*x[i] - q[i]
		}
		for r := 0; r < m; r++ {
			c := rhoV[r]*z[r] - y[r]
			for i := 0; i < n; i++ {
				if a := A.At(r, i); a != 0 {
					rhs[i] += a * c
				}
			}
		}
		if err := ch.SolveVecTo(&xt, mat.NewVecDense(n, rhs)); err != nil {
			return nil, it, err
		}
		zts := make([]float64, m) // z̃ = A·x̃
		for r := 0; r < m; r++ {
			s := 0.0
			for i := 0; i < n; i++ {
				s += A.At(r, i) * xt.AtVec(i)
			}
			zts[r] = s
		}
		for i := 0; i < n; i++ {
			x[i] = alpha*xt.AtVec(i) + (1-alpha)*x[i]
		}
		for r := 0; r < m; r++ {
			s := 0.0
			for i := 0; i < n; i++ {
				s += A.At(r, i) * x[i]
			}
			Ax[r] = s
		}
		for r := 0; r < m; r++ {
			zt := zts[r]
			zn := clip(alpha*zt+(1-alpha)*z[r]+y[r]/rhoV[r], l[r], u[r])
			y[r] += rhoV[r] * (alpha*zt + (1-alpha)*z[r] - zn)
			z[r] = zn
		}
		if it%25 != 0 {
			continue
		}
		var rp, rd, nAx, nz float64
		for r := 0; r < m; r++ {
			rp = math.Max(rp, math.Abs(Ax[r]-z[r]))
			nAx, nz = math.Max(nAx, math.Abs(Ax[r])), math.Max(nz, math.Abs(z[r]))
		}
		for i := 0; i < n; i++ {
			g := q[i]
			for j := 0; j < n; j++ {
				g += P.At(i, j) * x[j]
			}
			for r := 0; r < m; r++ {
				g += A.At(r, i) * y[r]
			}
			rd = math.Max(rd, math.Abs(g))
		}
		if rp <= tol*(1+math.Max(nAx, nz)) && rd <= tol*2 {
			return x, it, nil
		}
		if it%200 == 0 && rp > 0 && rd > 0 { // ρ adaptativo
			nr := rho * math.Sqrt((rp/(1+math.Max(nAx, nz)))/(rd/2))
			if nr > 5*rho || nr < rho/5 {
				rho = clip(nr, 1e-6, 1e6)
				if err := factor(); err != nil {
					return nil, it, err
				}
			}
		}
	}
	return x, maxIter, fmt.Errorf("el QP no convergió en %d iteraciones", maxIter)
}

// ErrSinExcesoPositivo: ningún activo rinde más que la tasa libre de riesgo, no hay portafolio de Sharpe positivo.
var ErrSinExcesoPositivo = errors.New("ningún activo supera la tasa libre de riesgo")

// MaxSharpe devuelve los pesos de máximo Sharpe long-only con tope maxW por activo y L2 opcional
// (gamma ≥ 0), por la transformación de Cornuejols–Tütüncü: con y = k·w,
//
//	min y'Σy + γ·y'y   s.t.  (μ-rf)'y = 1,  Σy = k,  0 ≤ y_i ≤ maxW·k,  k ≥ 0.
//
// Maximiza el Sharpe con el QP ADMM propio.
// Si ningún activo supera rf devuelve ErrSinExcesoPositivo (el problema transformado es infactible).
func MaxSharpe(mu []float64, cov *mat.Dense, rf, maxW, gamma float64) ([]float64, error) {
	n := len(mu)
	feasible := false
	for _, m := range mu {
		if m > rf {
			feasible = true
		}
	}
	if !feasible {
		return nil, ErrSinExcesoPositivo
	}
	N := n + 1 // y_1..y_n, k
	P := mat.NewDense(N, N, nil)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			v := 2 * cov.At(i, j)
			if i == j {
				v += 2 * gamma
			}
			P.Set(i, j, v)
		}
	}
	m := 2 + n + 1
	A := mat.NewDense(m, N, nil)
	l, u := make([]float64, m), make([]float64, m)
	for i := 0; i < n; i++ {
		A.Set(0, i, mu[i]-rf)
		A.Set(1, i, 1)
	}
	A.Set(1, n, -1)
	l[0], u[0], l[1], u[1] = 1, 1, 0, 0
	for i := 0; i < n; i++ {
		r := 2 + i // y_i − maxW·k ≤ 0 (y_i ≥ 0 va como cota de variable abajo)
		A.Set(r, i, 1)
		A.Set(r, n, -maxW)
		l[r], u[r] = math.Inf(-1), 0
	}
	A.Set(m-1, n, 1)
	l[m-1], u[m-1] = 0, math.Inf(1)
	// y_i ≥ 0: filas adicionales
	A2 := mat.NewDense(m+n, N, nil)
	A2.Copy(A)
	l2, u2 := append(append([]float64(nil), l...), make([]float64, n)...), append(append([]float64(nil), u...), make([]float64, n)...)
	for i := 0; i < n; i++ {
		A2.Set(m+i, i, 1)
		l2[m+i], u2[m+i] = 0, math.Inf(1)
	}
	x, _, err := QP(P, A2, make([]float64, N), l2, u2, 1e-10, 400_000)
	if err != nil {
		return nil, err
	}
	if x[n] <= 0 {
		return nil, errors.New("la optimización devolvió un portafolio degenerado")
	}
	w := make([]float64, n)
	for i := range w {
		w[i] = x[i] / x[n]
	}
	return w, nil
}
