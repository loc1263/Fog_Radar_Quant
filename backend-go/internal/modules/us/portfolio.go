package us

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"gonum.org/v1/gonum/mat"

	"fogradar/internal/core"
	"fogradar/internal/quant"
)

// Constantes del modelo de portafolio.
const (
	TradingDays         = 252
	MinTickers          = 2
	MinCommonDays       = 30
	DefaultRiskFree     = 0.04
	DefaultRiskAversion = 2.5  // δ: aversión al riesgo del equilibrio de mercado
	DefaultTau          = 0.05 // incertidumbre del prior de equilibrio en Black-Litterman
	DefaultBLViewScale  = 0.5  // k: cuánto tilt aplica el score de salud sobre el prior
	CovRidge            = 1e-6 // regularización contra covarianzas casi singulares
	DefaultNAssets      = 10
	MinNAssets          = 2
	MaxNAssets          = 20
	DefaultMaxWeight    = 0.25
	MinMaxWeight        = 0.05
)

// PortfolioDataError: datos insuficientes para armar el portafolio pedido (nunca dispara una descarga).
type PortfolioDataError struct{ Msg string }

func (e *PortfolioDataError) Error() string { return e.Msg }

func pderr(format string, a ...any) error { return &PortfolioDataError{fmt.Sprintf(format, a...)} }

// Returns son los retornos diarios por ticker (columnas, en orden).
type Returns struct {
	Tickers []string
	Cols    [][]float64
}

// LoadPriceMatrix lee prices.json y devuelve los cierres de tickers alineados por fecha (solo los días
// en que TODOS tienen dato), por columna.
func LoadPriceMatrix(runDir string, tickers []string) (*Returns, error) {
	if len(tickers) < MinTickers {
		return nil, pderr("Se necesitan al menos %d tickers para optimizar un portafolio.", MinTickers)
	}
	var m PriceMatrix
	if e := ReadRunFile(runDir, "prices.json", &m); e != nil {
		if e.Code == "no_data" {
			return nil, pderr("La corrida más reciente todavía no tiene datos de precios (prices.json). " +
				"Corré el batch de nuevo (fogradar job us_stocks) para generarlos.")
		}
		return nil, e
	}
	var missing []string
	for _, t := range tickers {
		if _, ok := m.Prices[t]; !ok {
			missing = append(missing, t)
		}
	}
	if len(missing) > 0 {
		return nil, pderr("Sin histórico de precios para: %s. Esperá a la próxima corrida del batch o elegí otros tickers.", join(missing))
	}
	// días con dato en todos los tickers
	var rows []int
	for i := range m.Dates {
		ok := true
		for _, t := range tickers {
			if i >= len(m.Prices[t]) || m.Prices[t][i] == nil {
				ok = false
				break
			}
		}
		if ok {
			rows = append(rows, i)
		}
	}
	if len(rows) < MinCommonDays {
		return nil, pderr("Histórico común insuficiente entre los tickers seleccionados (%d días, se necesitan al menos %d).", len(rows), MinCommonDays)
	}
	r := &Returns{Tickers: tickers}
	for _, t := range tickers {
		prices := make([]float64, len(rows))
		for k, i := range rows {
			prices[k] = *m.Prices[t][i]
		}
		r.Cols = append(r.Cols, quant.PctChange(prices))
	}
	if len(r.Cols[0]) == 0 || len(tickers) < MinTickers {
		return nil, pderr("No hay suficientes retornos diarios para calcular la covarianza.")
	}
	return r, nil
}

func join(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}

func (r *Returns) annualizedMean() []float64 {
	out := make([]float64, len(r.Cols))
	for i, c := range r.Cols {
		out[i] = quant.MeanOf(c) * TradingDays
	}
	return out
}

func (r *Returns) annualizedCov() *mat.Dense {
	c := quant.Cov(r.Cols)
	k, _ := c.Dims()
	for i := 0; i < k; i++ {
		for j := 0; j < k; j++ {
			v := c.At(i, j) * TradingDays
			if i == j {
				v += CovRidge
			}
			c.Set(i, j, v)
		}
	}
	return c
}

func (r *Returns) sub(tickers []string) *Returns {
	idx := map[string]int{}
	for i, t := range r.Tickers {
		idx[t] = i
	}
	out := &Returns{Tickers: tickers}
	for _, t := range tickers {
		out.Cols = append(out.Cols, r.Cols[idx[t]])
	}
	return out
}

// Pick es un activo elegido por cluster.
type Pick struct {
	Ticker  string
	Cluster int
	Score   *float64
}

// SelectByCluster elige nAssets tickers, uno por cluster de correlación (el de mayor score).
// Distancia d = √(0.5·(1−ρ)), enlace promedio, corte en nAssets clusters; si el corte da menos clusters
// se completa con los tickers de mayor score aún no elegidos. Devuelve la selección por score descendente.
func SelectByCluster(r *Returns, scores map[string]*float64, nAssets int) ([]Pick, error) {
	if nAssets < MinNAssets || nAssets > MaxNAssets {
		return nil, pderr("El portafolio debe tener entre %d y %d activos.", MinNAssets, MaxNAssets)
	}
	tickers := r.Tickers
	n := len(tickers)
	scoreOf := func(t string) float64 {
		if s := scores[t]; s != nil {
			return *s
		}
		return 0
	}
	less := func(a, b string) bool { // (−score, ticker)
		if scoreOf(a) != scoreOf(b) {
			return scoreOf(a) > scoreOf(b)
		}
		return a < b
	}

	labels := make([]int, n)
	if n <= nAssets {
		for i := range labels {
			labels[i] = i + 1
		}
	} else {
		corr := quant.Corr(r.Cols)
		dist := make([][]float64, n)
		for i := range dist {
			dist[i] = make([]float64, n)
			for j := range dist[i] {
				if i == j {
					continue
				}
				c := corr.At(i, j)
				if math.IsNaN(c) {
					c = 0
				}
				c = math.Max(-1, math.Min(1, c))
				dist[i][j] = math.Sqrt(0.5 * (1 - c))
			}
		}
		labels = quant.FclusterMaxclust(quant.AverageLinkage(dist), n, nAssets)
	}
	members := map[int][]string{}
	for i, t := range tickers {
		members[labels[i]] = append(members[labels[i]], t)
	}
	picks := map[string]bool{}
	for _, g := range members {
		best := g[0]
		for _, t := range g[1:] {
			if less(t, best) {
				best = t
			}
		}
		picks[best] = true
	}
	wanted := min(nAssets, n)
	if len(picks) < wanted {
		var rest []string
		for _, t := range tickers {
			if !picks[t] {
				rest = append(rest, t)
			}
		}
		sort.Slice(rest, func(i, j int) bool { return less(rest[i], rest[j]) })
		for _, t := range rest[:wanted-len(picks)] {
			picks[t] = true
		}
	}
	ordered := make([]string, 0, len(picks))
	for t := range picks {
		ordered = append(ordered, t)
	}
	sort.Slice(ordered, func(i, j int) bool { return less(ordered[i], ordered[j]) })
	out := make([]Pick, len(ordered))
	for i, t := range ordered {
		out[i] = Pick{Ticker: t, Cluster: i + 1, Score: scores[t]}
	}
	return out, nil
}

// BL es el resultado de Black-Litterman.
type BL struct {
	Weights   map[string]float64
	Prior     map[string]float64
	Views     map[string]float64
	Posterior map[string]float64
}

// BlackLittermanWeights: prior de equilibrio de mercado (reverse optimization, retornos totales = exceso + rf) +
// views automáticas derivadas del score de salud, mezcladas con la fórmula estándar de BL (P = I), y portafolio
// tangente long-only con tope de peso. views (opcional) reemplaza cualquiera de las automáticas.
func BlackLittermanWeights(r *Returns, caps, scores map[string]*float64, rf float64, views map[string]float64, maxWeight float64) (*BL, error) {
	tickers := r.Tickers
	n := len(tickers)
	cov := r.annualizedCov()

	wMkt := make([]float64, n)
	var total float64
	for i, t := range tickers {
		if c := caps[t]; c != nil && *c > 0 {
			wMkt[i] = *c
			total += *c
		}
	}
	for i := range wMkt {
		if total > 0 {
			wMkt[i] /= total
		} else {
			wMkt[i] = 1 / float64(n)
		}
	}

	pi := make([]float64, n)
	for i := 0; i < n; i++ {
		s := 0.0
		for j := 0; j < n; j++ {
			s += cov.At(i, j) * wMkt[j]
		}
		pi[i] = DefaultRiskAversion*s + rf
	}
	q := make([]float64, n)
	for i, t := range tickers {
		if v, ok := views[t]; ok {
			q[i] = v
			continue
		}
		score := 50.0
		if s := scores[t]; s != nil && *s != 0 {
			score = *s
		}
		q[i] = pi[i] + DefaultBLViewScale*(score/100-0.5)*math.Sqrt(cov.At(i, i))
	}

	// P = I: omega = diag(tau·diag(Σ)) + 1e-8·I
	tauSigma := mat.NewDense(n, n, nil)
	tauSigma.Scale(DefaultTau, cov)
	var tauSigmaInv mat.Dense
	if err := tauSigmaInv.Inverse(tauSigma); err != nil {
		return nil, pderr("No se pudo invertir la covarianza: %v", err)
	}
	omegaInv := mat.NewDense(n, n, nil)
	for i := 0; i < n; i++ {
		omegaInv.Set(i, i, 1/(DefaultTau*cov.At(i, i)+1e-8))
	}
	var prec mat.Dense
	prec.Add(&tauSigmaInv, omegaInv)
	var postCov mat.Dense
	if err := postCov.Inverse(&prec); err != nil {
		return nil, pderr("No se pudo invertir la precisión posterior: %v", err)
	}
	rhs := mat.NewVecDense(n, nil)
	var t1, t2 mat.VecDense
	t1.MulVec(&tauSigmaInv, mat.NewVecDense(n, pi))
	t2.MulVec(omegaInv, mat.NewVecDense(n, q))
	rhs.AddVec(&t1, &t2)
	var post mat.VecDense
	post.MulVec(&postCov, rhs)

	mu := make([]float64, n)
	for i := range mu {
		mu[i] = post.AtVec(i)
	}
	w, err := maxSharpeWeights(mu, cov, rf, maxWeight)
	if err != nil {
		return nil, err
	}
	out := &BL{Weights: map[string]float64{}, Prior: map[string]float64{}, Views: map[string]float64{}, Posterior: map[string]float64{}}
	for i, t := range tickers {
		out.Weights[t], out.Prior[t], out.Views[t], out.Posterior[t] = w[i], pi[i], q[i], mu[i]
	}
	return out, nil
}

func maxSharpeWeights(mu []float64, cov *mat.Dense, rf, maxWeight float64) ([]float64, error) {
	n := len(mu)
	cap := math.Max(maxWeight, 1/float64(n)) // con tope < 1/n no existe solución que sume 1
	w, err := quant.MaxSharpe(mu, cov, rf, cap, 0)
	if errors.Is(err, quant.ErrSinExcesoPositivo) {
		return nil, pderr("No se pudo optimizar el portafolio: ningún activo supera la tasa libre de riesgo (%.1f %%).", rf*100)
	}
	if err != nil {
		return nil, pderr("No se pudo optimizar el portafolio: %v", err)
	}
	sum := 0.0
	for i := range w {
		w[i] = math.Max(w[i], 0)
		sum += w[i]
	}
	if sum <= 0 {
		return nil, pderr("La optimización devolvió un portafolio degenerado (todos los pesos en cero).")
	}
	for i := range w {
		w[i] /= sum
	}
	return w, nil
}

// PortfolioMetricsOf calcula retorno esperado, volatilidad y Sharpe del portafolio.
func PortfolioMetricsOf(weights map[string]float64, r *Returns, rf float64) PortfolioMetrics {
	n := len(r.Tickers)
	w := make([]float64, n)
	for i, t := range r.Tickers {
		w[i] = weights[t]
	}
	mean, cov := r.annualizedMean(), r.annualizedCov()
	ret := 0.0
	for i := range w {
		ret += w[i] * mean[i]
	}
	v := 0.0
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			v += w[i] * cov.At(i, j) * w[j]
		}
	}
	vol := math.Sqrt(v)
	sharpe := 0.0
	if vol > 1e-12 {
		sharpe = (ret - rf) / vol
	}
	return PortfolioMetrics{ExpectedReturn: ret, Volatility: vol, Sharpe: sharpe}
}

// Built es el portafolio completo.
type Built struct {
	Selection []Pick
	BL        *BL
	Metrics   PortfolioMetrics
	MaxWeight float64
}

// BuildPortfolio aplica las etapas 2 (clusters) y 3 (Black-Litterman) sobre el universo de r.
func BuildPortfolio(r *Returns, caps, scores map[string]*float64, rf float64, views map[string]float64, nAssets int, maxWeight float64) (*Built, error) {
	if maxWeight < MinMaxWeight || maxWeight > 1 {
		return nil, pderr("El peso máximo por activo debe estar entre %d%% y 100%%.", int(MinMaxWeight*100))
	}
	sel, err := SelectByCluster(r, scores, nAssets)
	if err != nil {
		return nil, err
	}
	chosen := make([]string, len(sel))
	for i, s := range sel {
		chosen[i] = s.Ticker
	}
	sub := r.sub(chosen)
	var subViews map[string]float64
	for t, v := range views {
		for _, c := range chosen {
			if c == t {
				if subViews == nil {
					subViews = map[string]float64{}
				}
				subViews[t] = v
			}
		}
	}
	bl, err := BlackLittermanWeights(sub, caps, scores, rf, subViews, maxWeight)
	if err != nil {
		return nil, err
	}
	return &Built{Selection: sel, BL: bl, Metrics: PortfolioMetricsOf(bl.Weights, sub, rf),
		MaxWeight: math.Max(maxWeight, 1/float64(len(chosen)))}, nil
}

// MarketCapsAndScores lee market cap y score de cada ticker desde candidates.json.
func MarketCapsAndScores(runDir string, tickers []string) (caps, scores map[string]*float64, e *core.APIError) {
	var c RunResponse
	if e := ReadRunFile(runDir, "candidates.json", &c); e != nil {
		return nil, nil, e
	}
	by := map[string]CandidateRow{}
	for _, r := range c.Rows {
		by[r.Ticker] = r
	}
	caps, scores = map[string]*float64{}, map[string]*float64{}
	for _, t := range tickers {
		caps[t] = by[t].MarketCap
		scores[t] = by[t].Score.Overall
	}
	return caps, scores, nil
}
