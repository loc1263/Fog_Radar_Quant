package us

import (
	"strconv"
)

// CategoryWeights del score global.
var categoryOrder = []string{"valuation", "profitability", "financial_health", "shareholder_return", "technical"}

var categoryWeights = map[string]float64{
	"valuation": 0.25, "profitability": 0.25, "financial_health": 0.20, "shareholder_return": 0.10, "technical": 0.20,
}

type pt struct{ x, y float64 }

// piecewise interpola linealmente sobre una curva (x, score) ordenada por x; fuera de rango satura.
func piecewise(v *float64, points []pt) *float64 {
	if v == nil {
		return nil
	}
	val := *v
	switch {
	case val <= points[0].x:
		return fp(points[0].y)
	case val >= points[len(points)-1].x:
		return fp(points[len(points)-1].y)
	}
	for i := 0; i+1 < len(points); i++ {
		a, b := points[i], points[i+1]
		if a.x <= val && val <= b.x {
			t := (val - a.x) / (b.x - a.x)
			return fp(a.y + t*(b.y-a.y))
		}
	}
	return fp(points[len(points)-1].y)
}

func fp(v float64) *float64 { return &v }

func avg(scores ...*float64) *float64 {
	sum, n := 0.0, 0
	for _, s := range scores {
		if s != nil {
			sum += *s
			n++
		}
	}
	if n == 0 {
		return nil
	}
	return fp(sum / float64(n))
}

func negOr(v *float64, neg float64, curve []pt) *float64 {
	if v == nil {
		return nil
	}
	if *v < 0 {
		return fp(neg)
	}
	return piecewise(v, curve)
}

func scorePER(v *float64) *float64 {
	return negOr(v, 10, []pt{{5, 100}, {15, 90}, {25, 65}, {40, 35}, {60, 10}}) // ganancias negativas
}

func scorePEG(v *float64) *float64 {
	if v == nil {
		return nil
	}
	if *v <= 0 {
		return fp(30) // crecimiento negativo o dato poco fiable
	}
	return piecewise(v, []pt{{0.5, 100}, {1, 90}, {1.5, 70}, {2, 50}, {3, 25}, {5, 5}})
}

func scoreEVEBITDA(v *float64) *float64 {
	return negOr(v, 10, []pt{{5, 100}, {10, 90}, {15, 70}, {20, 50}, {30, 25}, {45, 5}}) // EBITDA negativo
}

func scorePriceToBook(v *float64) *float64 {
	return negOr(v, 5, []pt{{0.5, 90}, {1, 100}, {3, 85}, {6, 60}, {10, 35}, {20, 10}}) // patrimonio negativo
}

// ScoreValuation: múltiplos más bajos = mejor.
func ScoreValuation(v Valuation) *float64 {
	return avg(scorePER(v.PerTrailing), scorePEG(v.PegRatio), scoreEVEBITDA(v.EvToEbitda), scorePriceToBook(v.PriceToBook))
}

// ScoreProfitability: más alto = mejor.
func ScoreProfitability(p Profitability) *float64 {
	return avg(
		piecewise(p.Roe, []pt{{-0.2, 0}, {0, 25}, {0.08, 55}, {0.15, 75}, {0.25, 90}, {0.40, 100}}),
		piecewise(p.Roa, []pt{{-0.1, 0}, {0, 25}, {0.03, 50}, {0.06, 65}, {0.10, 80}, {0.15, 95}, {0.25, 100}}),
		piecewise(p.NetMargin, []pt{{-0.1, 0}, {0, 20}, {0.05, 45}, {0.10, 60}, {0.15, 75}, {0.20, 85}, {0.30, 100}}),
	)
}

// ScoreFinancialHealth: deuda, liquidez y FCF yield.
func ScoreFinancialHealth(h FinancialHealth, marketCap *float64) *float64 {
	var fcfYield *float64
	if h.FreeCashFlow != nil && marketCap != nil && *marketCap != 0 {
		y := *h.FreeCashFlow / *marketCap
		fcfYield = piecewise(&y, []pt{{-0.05, 5}, {0, 35}, {0.02, 60}, {0.04, 75}, {0.06, 88}, {0.10, 100}})
	}
	return avg(
		negOr(h.DebtToEquity, 5, []pt{{0, 100}, {50, 90}, {100, 70}, {150, 50}, {250, 25}, {400, 5}}),
		piecewise(h.CurrentRatio, []pt{{0.5, 15}, {1, 50}, {1.5, 75}, {2, 90}, {3, 100}}),
		fcfYield,
	)
}

// ScoreShareholderReturn: sube hasta un dividendo «saludable» (~5 %) y baja para yields muy altos.
func ScoreShareholderReturn(r ShareholderReturn) *float64 {
	return piecewise(r.DividendYield, []pt{{0, 45}, {0.02, 65}, {0.035, 85}, {0.05, 100}, {0.09, 70}, {0.15, 30}})
}

// ScoreTechnical: RSI, MACD y tendencia.
func ScoreTechnical(t Technical, price *float64) *float64 {
	var macd *float64
	if t.Macd.V != nil {
		h := t.Macd.V.Histogram
		macd = piecewise(&h, []pt{{-3, 10}, {0, 50}, {3, 90}}) // histograma > 0 = impulso alcista
	}
	var trend *float64
	if price != nil && t.Sma50 != nil && t.Sma200 != nil {
		hits := 0
		for _, c := range []bool{*price > *t.Sma50, *price > *t.Sma200, *t.Sma50 > *t.Sma200} {
			if c {
				hits++
			}
		}
		trend = fp([]float64{10, 35, 70, 100}[hits])
	}
	return avg(piecewise(t.Rsi14, []pt{{20, 25}, {30, 40}, {45, 80}, {60, 95}, {70, 75}, {80, 40}, {90, 15}}), macd, trend)
}

func label(score float64) string {
	switch {
	case score >= 75:
		return "Fuerte"
	case score >= 60:
		return "Sólido"
	case score >= 45:
		return "Neutral"
	case score >= 30:
		return "Débil"
	}
	return "Muy débil"
}

// round1 redondea a 1 decimal como round(x, 1) de Python (correctamente redondeado, mitades al par).
func round1(x float64) float64 {
	v, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 1, 64), 64)
	return v
}

func r1(v *float64) *float64 {
	if v == nil {
		return nil
	}
	return fp(round1(*v))
}

// Evaluation es lo que necesita evaluate_score: los bloques ya armados de un ticker.
type Evaluation struct {
	Price             *float64
	MarketCap         *float64
	Valuation         Valuation
	Profitability     Profitability
	FinancialHealth   FinancialHealth
	ShareholderReturn ShareholderReturn
	Technical         Technical
}

// EvaluateScore calcula el score de salud (0-100): cada categoría se promedia sobre las métricas
// disponibles; el global es el promedio ponderado de las categorías disponibles, renormalizando los pesos.
func EvaluateScore(e Evaluation) Score {
	cats := map[string]*float64{
		"valuation":          ScoreValuation(e.Valuation),
		"profitability":      ScoreProfitability(e.Profitability),
		"financial_health":   ScoreFinancialHealth(e.FinancialHealth, e.MarketCap),
		"shareholder_return": ScoreShareholderReturn(e.ShareholderReturn),
		"technical":          ScoreTechnical(e.Technical, e.Price),
	}
	sum, wt := 0.0, 0.0
	for _, k := range categoryOrder {
		if s := cats[k]; s != nil {
			sum += *s * categoryWeights[k]
			wt += categoryWeights[k]
		}
	}
	out := Score{Label: "Sin datos", Categories: ScoreCategories{
		Valuation: r1(cats["valuation"]), Profitability: r1(cats["profitability"]),
		FinancialHealth: r1(cats["financial_health"]), ShareholderReturn: r1(cats["shareholder_return"]),
		Technical: r1(cats["technical"]),
	}}
	if wt > 0 {
		o := round1(sum / wt)
		out.Overall = &o
		out.Label = label(o)
	}
	return out
}
