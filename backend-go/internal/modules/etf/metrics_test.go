package etf

import (
	"math"
	"testing"
	"time"

	"github.com/loc1263/gohoo"
)

func barsAdj(vals ...float64) []gohoo.Bar {
	out := make([]gohoo.Bar, len(vals))
	t0 := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	for i, v := range vals {
		out[i] = gohoo.Bar{Time: t0.AddDate(0, 0, i), Close: v, AdjClose: v}
	}
	return out
}

func TestRetornosDescartaNaNYCeros(t *testing.T) {
	dates, r := retornos(barsAdj(100, 110, math.NaN(), 120, 0, 50, 55))
	// válidos: 110/100, 55/50 (los que tocan el NaN o salen de 0 se descartan; 0/120−1 = −1 sí vale)
	if len(r) != 3 || math.Abs(r[0]-0.1) > 1e-12 || r[1] != -1 || math.Abs(r[2]-0.1) > 1e-12 {
		t.Fatalf("r = %v", r)
	}
	if dates[0] != "2026-01-06" || dates[1] != "2026-01-09" || dates[2] != "2026-01-11" {
		t.Fatalf("fechas = %v", dates)
	}
}

func TestSharpeValorYBordes(t *testing.T) {
	// retornos +1 % y −1 % alternados: media 0 → Sharpe = −rf / (desvío·√252).
	b := barsAdj(100, 101, 99.99, 100.9899)
	_, r := retornos(b)
	mean := (r[0] + r[1] + r[2]) / 3
	var ss float64
	for _, x := range r {
		ss += (x - mean) * (x - mean)
	}
	want := math.Round(((mean*tradingDays)-riskFreeRate)/(math.Sqrt(ss/2)*math.Sqrt(tradingDays))*1000) / 1000
	if s := Sharpe(b); s == nil || *s != want {
		t.Fatalf("Sharpe = %v, want %v", s, want)
	}
	if Sharpe(barsAdj(100, 100, 100)) != nil {
		t.Error("desvío 0 debe dar nil")
	}
	if Sharpe(barsAdj(100, 101)) != nil {
		t.Error("menos de 2 retornos debe dar nil")
	}
}
