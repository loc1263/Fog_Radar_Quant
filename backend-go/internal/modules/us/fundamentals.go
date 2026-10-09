package us

import (
	"math"

	"github.com/loc1263/gohoo"

	"fogradar/internal/core"
)

// num convierte un valor de info a float64; nil si falta o no es número.
func num(v any) *float64 {
	f, ok := gohoo.AsFloat(v)
	if !ok || math.IsNaN(f) {
		return nil
	}
	return &f
}

// truthy dice si un número es verdadero (0 es falso).
func truthy(v *float64) bool { return v != nil && *v != 0 }

// BuildValuation arma los múltiplos de valoración desde Ticker.info.
func BuildValuation(info map[string]any) Valuation {
	peg := num(info["trailingPegRatio"])
	if !truthy(peg) {
		peg = num(info["pegRatio"])
	}
	return Valuation{
		PerTrailing: num(info["trailingPE"]), PerForward: num(info["forwardPE"]), PegRatio: peg,
		EvToEbitda: num(info["enterpriseToEbitda"]), PriceToBook: num(info["priceToBook"]),
		EpsTrailing: num(info["trailingEps"]), EpsForward: num(info["forwardEps"]),
	}
}

// BuildProfitability arma la rentabilidad.
func BuildProfitability(info map[string]any) Profitability {
	return Profitability{Roe: num(info["returnOnEquity"]), Roa: num(info["returnOnAssets"]), NetMargin: num(info["profitMargins"])}
}

// BuildFinancialHealth arma la salud financiera.
func BuildFinancialHealth(info map[string]any) FinancialHealth {
	return FinancialHealth{DebtToEquity: num(info["debtToEquity"]), CurrentRatio: num(info["currentRatio"]),
		FreeCashFlow: num(info["freeCashflow"])}
}

// BuildShareholderReturn: info trae dividendYield en porcentaje (5.68 = 5,68 %); se normaliza a fracción,
// consistente con el resto de los porcentajes de la app.
func BuildShareholderReturn(info map[string]any) ShareholderReturn {
	raw := num(info["dividendYield"])
	if raw == nil {
		return ShareholderReturn{}
	}
	v := *raw / 100
	return ShareholderReturn{DividendYield: &v}
}

// --- indicadores técnicos ---

// SMA de la última ventana; nil si hay menos datos que la ventana.
func SMA(closes []float64, window int) *float64 {
	if len(closes) < window {
		return nil
	}
	sum := 0.0
	tail := closes[len(closes)-window:]
	for _, v := range tail {
		sum += v
	}
	m := sum / float64(window)
	return &m
}

// RSI de Wilder (ewm con alpha=1/window, adjust=False) del último día.
func RSI(closes []float64, window int) *float64 {
	if len(closes) < window+1 {
		return nil
	}
	alpha := 1 / float64(window)
	var ag, al float64
	for i := 1; i < len(closes); i++ {
		d := closes[i] - closes[i-1]
		gain, loss := math.Max(d, 0), math.Max(-d, 0)
		if i == 1 {
			ag, al = gain, loss
		} else {
			ag = (1-alpha)*ag + alpha*gain
			al = (1-alpha)*al + alpha*loss
		}
	}
	if math.IsNaN(ag) || math.IsNaN(al) {
		return nil
	}
	if al == 0 {
		return fp(100)
	}
	rs := ag / al
	return fp(100 - (100 / (1 + rs)))
}

func ewm(xs []float64, span int) []float64 {
	alpha := 2 / float64(span+1)
	out := make([]float64, len(xs))
	for i, x := range xs {
		if i == 0 {
			out[i] = x
		} else {
			out[i] = (1-alpha)*out[i-1] + alpha*x
		}
	}
	return out
}

// MACD (12, 26, 9) del último día; nil con menos de slow+signal datos.
func MACD(closes []float64) core.Opt[Macd] {
	const fast, slow, signal = 12, 26, 9
	if len(closes) < slow+signal {
		return core.Opt[Macd]{}
	}
	ef, es := ewm(closes, fast), ewm(closes, slow)
	line := make([]float64, len(closes))
	for i := range line {
		line[i] = ef[i] - es[i]
	}
	sig := ewm(line, signal)
	last := len(closes) - 1
	m, s := line[last], sig[last]
	h := m - s
	if math.IsNaN(m) || math.IsNaN(s) || math.IsNaN(h) {
		return core.Opt[Macd]{}
	}
	return core.Some(Macd{Macd: m, Signal: s, Histogram: h})
}

// BuildTechnical arma los indicadores técnicos. closes ya sin NaN; volumes puede traer NaN (se descartan).
func BuildTechnical(closes, volumes []float64) Technical {
	t := Technical{Sma50: SMA(closes, 50), Sma200: SMA(closes, 200), Rsi14: RSI(closes, 14), Macd: MACD(closes)}
	var vols []float64
	for _, v := range volumes {
		if !math.IsNaN(v) {
			vols = append(vols, v)
		}
	}
	if len(vols) > 0 {
		last := vols[len(vols)-1]
		t.Volume = &last
		tail := vols
		if len(tail) > 30 {
			tail = tail[len(tail)-30:]
		}
		sum := 0.0
		for _, v := range tail {
			sum += v
		}
		m := sum / float64(len(tail))
		t.AverageVolume = &m
	}
	return t
}
