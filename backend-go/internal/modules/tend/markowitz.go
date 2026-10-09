package tend

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"gonum.org/v1/gonum/mat"

	"fogradar/internal/quant"
)

// Selección de las mejores acciones vía el modelo de Markowitz (media-varianza).
//
//  1. Con los retornos semanales del universo se estiman el retorno esperado anualizado (CAGR) y la covarianza anualizada.
//  2. Se optimiza el portafolio de máximo Sharpe long-only con tope de concentración y regularización L2.
//  3. Se seleccionan los top_n activos de mayor peso.
//  4. Con esos se re-optimiza (tope 30 %, o 1/N) → portafolio final. Más portafolios reducidos (5 y 10).
//
// El solver es un QP ADMM propio (quant.MaxSharpe) (ADMM).

// Spec de un análisis: key de archivo, etiqueta, universo, cuántas acciones seleccionar y mínimo de tickers.
type Spec struct {
	Key, Label string
	Tickers    []string
	TopN       int
	MinTickers int
}

// Analyses devuelve los análisis configurados (el principal Asia + EEUU y el complementario solo EEUU).
func Analyses(u *Universe, p Params) []Spec {
	return []Spec{
		{"", "Asia + EEUU", u.All(), p.TopN, 1},
		{"_us", "solo EEUU", u.US, p.TopNUS, p.MinUSTickers},
	}
}

// Validar devuelve los problemas de la configuración (vacío si es factible): tope × top_n ≥ 1, top_n ≤ universo y los
// portafolios reducidos menores que top_n.
func Validar(u *Universe, p Params) []string {
	var out []string
	if !(p.MaxWeight > 0 && p.MaxWeight <= 1) {
		out = append(out, fmt.Sprintf("MAX_WEIGHT_PER_ASSET=%v debe estar en (0, 1].", p.MaxWeight))
	}
	for _, s := range Analyses(u, p) {
		if float64(s.TopN)*p.MaxWeight < 1 {
			out = append(out, fmt.Sprintf("[%s] top_n=%d × MAX_WEIGHT_PER_ASSET=%v < 1: los pesos no pueden sumar 100 %%.", s.Label, s.TopN, p.MaxWeight))
		}
		if len(s.Tickers) < s.TopN && len(s.Tickers) >= s.MinTickers {
			out = append(out, fmt.Sprintf("[%s] top_n=%d es mayor que el universo (%d tickers).", s.Label, s.TopN, len(s.Tickers)))
		}
		for _, n := range p.PortfolioSizes {
			if n >= s.TopN {
				out = append(out, fmt.Sprintf("[%s] portafolio reducido de %d >= top_n=%d.", s.Label, n, s.TopN))
			}
		}
	}
	return out
}

// Opt es el resultado de optimizar: pesos limpios (cutoff 1e-4, 5 decimales), rendimiento con los pesos crudos, mu y Σ.
type Opt struct {
	Weights []float64 // limpios
	Perf    [3]float64
	Mu      []float64
	Cov     *mat.Dense
}

// OptimizeMaxSharpe: máx. Sharpe con tope y L2; si falla (p. ej. ningún activo supera rf) cae a mínima volatilidad
// (sin L2, como el original).
func OptimizeMaxSharpe(prices *Prices, rf, ub, gamma float64, p Params, warn func(string, ...any)) (*Opt, error) {
	rets := quant.ReturnsFromPrices(prices.Cols)
	mu := quant.MeanHistoricalReturn(rets, float64(p.WeeksPerYear))
	S := quant.SampleCov(rets, float64(p.WeeksPerYear))
	w, err := quant.MaxSharpe(mu, S, rf, ub, gamma)
	if err != nil {
		warn("max_sharpe falló (%v); se intenta min_volatility como respaldo.", err)
		w, err = quant.MinVolatility(S, ub, 0)
		if err != nil {
			return nil, fmt.Errorf("no se pudo optimizar: %w", err)
		}
	} else {
		for i := range w {
			w[i] = math.Round(w[i]*1e16) / 1e16
		}
	}
	ret, vol, sh := quant.PortfolioPerformance(w, mu, S, rf)
	return &Opt{Weights: quant.CleanWeights(w, 1e-4, 5), Perf: [3]float64{ret, vol, sh}, Mu: mu, Cov: S}, nil
}

type ranked struct {
	ticker string
	w      float64
}

// sortDesc ordena por peso descendente (estable: los empates quedan en el orden de las columnas).
func sortDesc(tickers []string, w []float64) []ranked {
	r := make([]ranked, len(tickers))
	for i := range tickers {
		r[i] = ranked{tickers[i], w[i]}
	}
	sort.SliceStable(r, func(i, j int) bool { return r[i].w > r[j].w })
	return r
}

// SelectTopN devuelve los top_n tickers por peso (> 1e-6); si el optimizador dejó menos activos con peso, completa con los
// de mayor Sharpe individual entre los restantes.
func SelectTopN(order []ranked, tickers []string, mu []float64, cov *mat.Dense, rf float64, topN int) []string {
	var top []string
	for _, r := range order {
		if r.w > 1e-6 {
			top = append(top, r.ticker)
			if len(top) == topN {
				break
			}
		}
	}
	if len(top) < topN {
		type is struct {
			t string
			s float64
		}
		var list []is
		for i, t := range tickers {
			list = append(list, is{t, (mu[i] - rf) / math.Sqrt(cov.At(i, i))})
		}
		sort.SliceStable(list, func(a, b int) bool { return list[a].s > list[b].s })
		for _, x := range list {
			if len(top) >= topN {
				break
			}
			if !has(top, x.t) {
				top = append(top, x.t)
			}
		}
	}
	return top
}

// FinalResult es un portafolio final (peso_final/nombre/país) y su desempeño.
type FinalResult struct {
	Tickers []string // ordenados por peso desc
	Weights []float64
	Perf    [3]float64
}

// OptimizeFinal: máx. Sharpe sobre un grupo chico ya seleccionado, sin el tope del universo amplio (30 %, o 1/N si con 30 % no suma 100 %), L2 0.05.
func OptimizeFinal(prices *Prices, rf float64, p Params, warn func(string, ...any)) (*FinalResult, error) {
	ub := math.Max(0.30, 1/float64(len(prices.Tickers)))
	o, err := OptimizeMaxSharpe(prices, rf, ub, 0.05, p, warn)
	if err != nil {
		return nil, err
	}
	ord := sortDesc(prices.Tickers, o.Weights)
	fr := &FinalResult{Perf: o.Perf}
	for _, r := range ord {
		fr.Tickers, fr.Weights = append(fr.Tickers, r.ticker), append(fr.Weights, r.w)
	}
	return fr, nil
}

func writeFrameFile(path string, f *quant.Frame) error {
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := f.WriteCSV(fh); err != nil {
		fh.Close()
		return err
	}
	return fh.Close()
}

func strCol(name string, v []string) *quant.Column {
	return &quant.Column{Name: name, Kind: quant.Str, S: v, Null: make([]bool, len(v))}
}

func fCol(name string, v []float64) *quant.Column {
	return &quant.Column{Name: name, Kind: quant.Float, F: v}
}

// indexed escribe un CSV cuyo primer columna es el índice (ticker) sin nombre, como DataFrame.to_csv() con índice.
func writeIndexed(path string, tickers []string, cols ...*quant.Column) error {
	f := quant.NewFrame(len(tickers))
	f.Set(strCol("", tickers))
	for _, c := range cols {
		f.Set(c)
	}
	return writeFrameFile(path, f)
}

func perfMetrics(p [3]float64) OM {
	return OM{{"retorno_esperado_anual", p[0]}, {"volatilidad_anual", p[1]}, {"sharpe_ratio", p[2]}}
}

// Selection es el resultado de un análisis de Markowitz.
type Selection struct {
	Spec        Spec
	TopTickers  []string
	Final       *FinalResult
	Performance OM
}

func (u *Universe) names(tickers []string) (nombres, paises []string) {
	for _, t := range tickers {
		nombres, paises = append(nombres, u.Info[t].Name), append(paises, u.Info[t].Country)
	}
	return
}

func writeFile(path string, b []byte) error { return os.WriteFile(path, b, 0o644) }

// RunSelection corre la selección de Markowitz completa de un análisis y escribe sus archivos (con el sufijo del análisis):
// markowitz_universe_weights, top15_seleccion, portfolio_final_weights, portfolio_performance y los portafolios reducidos.
func RunSelection(prices *Prices, rf float64, runDir string, u *Universe, spec Spec, p Params, warn func(string, ...any)) (*Selection, error) {
	var cols []string
	for _, t := range prices.Tickers {
		if has(spec.Tickers, t) {
			cols = append(cols, t)
		}
	}
	pu := prices.Subset(cols)

	full, err := OptimizeMaxSharpe(pu, rf, p.MaxWeight, p.L2Gamma, p, warn)
	if err != nil {
		return nil, err
	}
	order := sortDesc(pu.Tickers, full.Weights)
	muOf := map[string]float64{}
	for i, t := range pu.Tickers {
		muOf[t] = full.Mu[i]
	}
	var uTickers []string
	var uW, uMu []float64
	for _, r := range order {
		uTickers, uW, uMu = append(uTickers, r.ticker), append(uW, r.w), append(uMu, muOf[r.ticker])
	}
	nom, pais := u.names(uTickers)
	suffix := spec.Key
	if err := writeIndexed(filepath.Join(runDir, "markowitz_universe_weights"+suffix+".csv"), uTickers,
		fCol("peso_markowitz", uW), strCol("nombre", nom), strCol("pais", pais), fCol("retorno_esperado_anual", uMu)); err != nil {
		return nil, err
	}

	top := SelectTopN(order, pu.Tickers, full.Mu, full.Cov, rf, spec.TopN)
	wOf := map[string]float64{}
	for _, r := range order {
		wOf[r.ticker] = r.w
	}
	var tw, tm []float64
	for _, t := range top {
		tw, tm = append(tw, wOf[t]), append(tm, muOf[t])
	}
	tn, tp := u.names(top)
	if err := writeIndexed(filepath.Join(runDir, "top15_seleccion"+suffix+".csv"), top,
		fCol("peso_markowitz", tw), strCol("nombre", tn), strCol("pais", tp), fCol("retorno_esperado_anual", tm)); err != nil {
		return nil, err
	}

	final, err := OptimizeFinal(pu.Subset(top), rf, p, warn)
	if err != nil {
		return nil, err
	}
	fn, fp := u.names(final.Tickers)
	if err := writeIndexed(filepath.Join(runDir, "portfolio_final_weights"+suffix+".csv"), final.Tickers,
		fCol("peso_final", final.Weights), strCol("nombre", fn), strCol("pais", fp)); err != nil {
		return nil, err
	}
	perf := OM{{"universo_completo", perfMetrics(full.Perf)}, {"portafolio_15_finales", perfMetrics(final.Perf)},
		{"tasa_libre_riesgo_anual", rf}, {"tickers_seleccionados", top}}
	if err := writeFile(filepath.Join(runDir, "portfolio_performance"+suffix+".json"), OrderedJSON(perf)); err != nil {
		return nil, err
	}

	// Portafolios reducidos: de las N de mayor peso, re-optimiza solo sobre ellas.
	for _, n := range p.PortfolioSizes {
		if n >= len(top) {
			continue
		}
		red, err := OptimizeFinal(pu.Subset(top[:n]), rf, p, warn)
		if err != nil {
			return nil, err
		}
		rn, rp := u.names(red.Tickers)
		sn := strconv.Itoa(n)
		if err := writeIndexed(filepath.Join(runDir, "portfolio_final_weights_top"+sn+suffix+".csv"), red.Tickers,
			fCol("peso_final", red.Weights), strCol("nombre", rn), strCol("pais", rp)); err != nil {
			return nil, err
		}
		perfN := OM{{"n_acciones", n}, {"retorno_esperado_anual", red.Perf[0]}, {"volatilidad_anual", red.Perf[1]},
			{"sharpe_ratio", red.Perf[2]}, {"tickers_seleccionados", append([]string(nil), top[:n]...)}}
		if err := writeFile(filepath.Join(runDir, "portfolio_performance_top"+sn+suffix+".json"), OrderedJSON(perfN)); err != nil {
			return nil, err
		}
	}
	return &Selection{Spec: spec, TopTickers: top, Final: final, Performance: perf}, nil
}

// ErrSinAnalisisPrincipal: el análisis principal no se pudo correr.
var ErrSinAnalisisPrincipal = errors.New("no se pudo correr el análisis principal")
