// Spike C (Fase 0): max-Sharpe con L2 y topes (PyPortfolioOpt) reproducido en Go con un QP ADMM (estilo OSQP)
// sobre gonum. Compara contra los pesos de PyPortfolioOpt exportados en fixture.json.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"gonum.org/v1/gonum/mat"
)

type Caso struct {
	Nombre  string               `json:"nombre"`
	Tickers []string             `json:"tickers"`
	Mu      []float64            `json:"mu"`
	S       [][]float64          `json:"S"`
	Rf      float64              `json:"rf"`
	Lb     float64              `json:"lb"`
	UbF     float64              `json:"ub"`
	Gamma   float64              `json:"gamma"`
	WRaw    map[string]float64   `json:"w_raw"`
	WClean  map[string]float64   `json:"w_clean"`
	Perf    []float64            `json:"perf"`
}

// qp resuelve min ½x'Px + q'x  s.t. l ≤ Ax ≤ u con ADMM (OSQP, rho adaptativo).
func qp(P, A *mat.Dense, q, l, u []float64, tol float64, maxIter int) ([]float64, int) {
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
	factor := func() {
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
			panic("KKT no definida positiva")
		}
	}
	factor()
	x, z, y := make([]float64, n), make([]float64, m), make([]float64, m)
	clip := func(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
	Ax := make([]float64, m)
	for it := 1; it <= maxIter; it++ {
		rhs := make([]float64, n)
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
		var xt mat.VecDense
		if err := ch.SolveVecTo(&xt, mat.NewVecDense(n, rhs)); err != nil {
			panic(err)
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
		if it%25 == 0 {
			var rp, rd, nAx, nz float64
			for r := 0; r < m; r++ {
				rp = math.Max(rp, math.Abs(Ax[r]-z[r]))
				nAx, nz = math.Max(nAx, math.Abs(Ax[r])), math.Max(nz, math.Abs(z[r]))
			}
			g := make([]float64, n)
			for i := 0; i < n; i++ {
				g[i] = q[i]
				for j := 0; j < n; j++ {
					g[i] += P.At(i, j) * x[j]
				}
				for r := 0; r < m; r++ {
					g[i] += A.At(r, i) * y[r]
				}
				rd = math.Max(rd, math.Abs(g[i]))
			}
			if rp <= tol*(1+math.Max(nAx, nz)) && rd <= tol*(1+1) {
				return x, it
			}
			if it%200 == 0 && rp > 0 && rd > 0 { // rho adaptativo
				nr := rho * math.Sqrt((rp/(1+math.Max(nAx, nz)))/(rd/2))
				if nr > 5*rho || nr < rho/5 {
					rho = clip(nr, 1e-6, 1e6)
					factor()
				}
			}
		}
	}
	return x, maxIter
}

func maxSharpe(c Caso) (map[string]float64, int) {
	n := len(c.Mu)
	N := n + 1 // y_1..y_n, k
	P := mat.NewDense(N, N, nil)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			v := 2 * c.S[i][j]
			if i == j {
				v += 2 * c.Gamma
			}
			P.Set(i, j, v)
		}
	}
	m := 2 + 2*n + 1
	A := mat.NewDense(m, N, nil)
	l, u := make([]float64, m), make([]float64, m)
	for i := 0; i < n; i++ {
		A.Set(0, i, c.Mu[i]-c.Rf)
		A.Set(1, i, 1)
	}
	A.Set(1, n, -1)
	l[0], u[0], l[1], u[1] = 1, 1, 0, 0
	for i := 0; i < n; i++ {
		r := 2 + i // y_i - ub k ≤ 0
		A.Set(r, i, 1)
		A.Set(r, n, -c.UbF)
		l[r], u[r] = math.Inf(-1), 0
		r = 2 + n + i // y_i - lb k ≥ 0
		A.Set(r, i, 1)
		A.Set(r, n, -c.Lb)
		l[r], u[r] = 0, math.Inf(1)
	}
	A.Set(m-1, n, 1)
	l[m-1], u[m-1] = 0, math.Inf(1)
	x, it := qp(P, A, make([]float64, N), l, u, 1e-10, 400000)
	w := map[string]float64{}
	for i, t := range c.Tickers {
		w[t] = x[i] / x[n]
	}
	return w, it
}

func main() {
	var casos []Caso
	b, err := os.ReadFile("fixture.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(b, &casos); err != nil {
		panic(err)
	}
	for _, c := range casos {
		t0 := time.Now()
		w, it := maxSharpe(c)
		el := time.Since(t0)
		var maxRaw, maxClean, suma float64
		var tk []string
		for t, v := range w {
			maxRaw = math.Max(maxRaw, math.Abs(v-c.WRaw[t]))
			clean := math.Round(v*1e5) / 1e5
			if math.Abs(clean) < 1e-4 { // clean_weights de PyPortfolioOpt: cutoff 1e-4, redondeo 5 dec.
				clean = 0
			}
			maxClean = math.Max(maxClean, math.Abs(clean-c.WClean[t]))
			suma += v
			tk = append(tk, t)
		}
		sort.Slice(tk, func(i, j int) bool { return w[tk[i]] > w[tk[j]] })
		fmt.Printf("%-9s n=%d iter=%d tiempo=%v suma=%.6f\n  max|Δw crudo|=%.2e  max|Δw limpio(5dec)|=%.2e\n  top3 Go: ", c.Nombre, len(c.Mu), it, el, suma, maxRaw, maxClean)
		for _, t := range tk[:3] {
			fmt.Printf("%s=%.5f (py %.5f)  ", t, w[t], c.WRaw[t])
		}
		fmt.Println()
	}
}
