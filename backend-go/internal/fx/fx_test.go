package fx

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

type fake struct {
	prices map[string]float64
	errs   map[string]error
	calls  []string
}

func (f *fake) src(_ context.Context, pair string) (float64, bool, error) {
	f.calls = append(f.calls, pair)
	if e := f.errs[pair]; e != nil {
		return 0, false, e
	}
	p, ok := f.prices[pair]
	return p, ok, nil
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-12 }

func TestMismaMonedaEs1(t *testing.T) {
	f := &fake{}
	if r, ok := NewRates(f.src, nil).Rate(context.Background(), "CLP", "CLP"); !ok || r != 1 || len(f.calls) != 0 {
		t.Fatal(r, ok, f.calls)
	}
}

func TestUSDaCLPyVuelta(t *testing.T) {
	r := NewRates((&fake{prices: map[string]float64{"CLP=X": 950}}).src, nil)
	if v, ok := r.Rate(context.Background(), "USD", "CLP"); !ok || v != 950 {
		t.Fatal(v, ok)
	}
	if v, ok := r.Rate(context.Background(), "CLP", "USD"); !ok || !near(v, 1.0/950) {
		t.Fatal(v, ok)
	}
}

func TestCruceViaUSD(t *testing.T) {
	r := NewRates((&fake{prices: map[string]float64{"EUR=X": 0.9, "CLP=X": 950}}).src, nil)
	if v, ok := r.Rate(context.Background(), "EUR", "CLP"); !ok || !near(v, 950/0.9) {
		t.Fatal(v, ok)
	}
}

func TestCacheaIncluidoFallos(t *testing.T) {
	f := &fake{errs: map[string]error{"CLP=X": errors.New("bloqueo de Yahoo")}}
	warns := 0
	r := NewRates(f.src, func(string, ...any) { warns++ })
	for i := 0; i < 2; i++ {
		if _, ok := r.Rate(context.Background(), "USD", "CLP"); ok {
			t.Fatal("debía fallar")
		}
	}
	if len(f.calls) != 1 || warns != 1 {
		t.Fatalf("calls=%v warns=%d", f.calls, warns)
	}
}

func TestPrecioCeroOAusenteNoEsTasa(t *testing.T) {
	r := NewRates((&fake{prices: map[string]float64{"CLP=X": 0}}).src, nil)
	if _, ok := r.Rate(context.Background(), "CLP", "USD"); ok {
		t.Fatal("precio 0")
	}
	if _, ok := NewRates((&fake{}).src, nil).Rate(context.Background(), "EUR", "CLP"); ok {
		t.Fatal("sin datos")
	}
}

func TestNormalizeYUSDPair(t *testing.T) {
	cases := []struct {
		in      string
		iso     string
		div     float64
		wantOK  bool
		usdPair string
	}{
		{"JPY", "JPY", 1, true, "USDJPY=X"},
		{"GBp", "GBP", 100, true, "USDGBP=X"},
		{"ZZZ", "", 0, false, "USDZZZ=X"},
		{"jpy", "", 0, false, "USDJPY=X"}, // Yahoo usa mayúsculas; minúscula = subunidad
		{"", "", 0, false, "USD=X"},
	}
	for _, c := range cases {
		iso, div, ok := Normalize(c.in)
		if iso != c.iso || div != c.div || ok != c.wantOK {
			t.Errorf("Normalize(%q) = %s %v %v", c.in, iso, div, ok)
		}
		if got := USDPair(c.in); got != c.usdPair {
			t.Errorf("USDPair(%q) = %s", c.in, got)
		}
	}
}

func day(n int) time.Time { return time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 7*n) }

func TestToUSDForwardFillYSubunidad(t *testing.T) {
	precios := Series{T: []time.Time{day(0), day(1), day(2)}, V: []float64{1000, 1100, 1200}}
	// Falta el FX de la última semana: se usa el último conocido (sin mirar al futuro).
	tasa := Series{T: []time.Time{day(0), day(1)}, V: []float64{0.8, 0.8}}
	got := ToUSD(precios, tasa, "GBp").V
	for i, w := range []float64{12.5, 13.75, 15} {
		if !near(got[i], w) {
			t.Fatalf("got %v", got)
		}
	}
}

func TestToUSDSemanticaDePandas(t *testing.T) {
	// FX con fecha que NO coincide con ninguna de precios: pandas la ignora (reindex exacto + ffill).
	precios := Series{T: []time.Time{day(0), day(1)}, V: []float64{10, 10}}
	fx := Series{T: []time.Time{day(0).Add(24 * time.Hour)}, V: []float64{2}}
	got := ToUSD(precios, fx, "JPY").V
	if !math.IsNaN(got[0]) || !math.IsNaN(got[1]) {
		t.Fatalf("sin coincidencia exacta debe dar NaN: %v", got)
	}
	// Antes del primer FX conocido → NaN, después se arrastra.
	fx = Series{T: []time.Time{day(1)}, V: []float64{2}}
	got = ToUSD(precios, fx, "JPY").V
	if !math.IsNaN(got[0]) || got[1] != 5 {
		t.Fatalf("%v", got)
	}
}
