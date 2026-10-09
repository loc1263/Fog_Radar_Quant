// Package fx son las tasas de cambio: spot con caché por par,
// normalización de monedas de Yahoo (subunidades GBp/ZAc/ILA) y conversión de series a USD.
//
// Convención de Yahoo: el ticker XXX=X vale «cuántas unidades de XXX compra 1 USD».
package fx

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/loc1263/gohoo"

	"fogradar/internal/yahoo"
)

// PriceSource entrega el último cierre de un par (CLP=X); ok=false si no hay dato.
type PriceSource func(ctx context.Context, pair string) (price float64, ok bool, err error)

// YahooPrice es la fuente real: último cierre de los últimos 5 días.
func YahooPrice(src yahoo.Source) PriceSource {
	return func(ctx context.Context, pair string) (float64, bool, error) {
		res, err := src.Chart(ctx, pair, gohoo.ChartParams{Range: "5d", Interval: "1d"})
		if err != nil {
			return 0, false, err
		}
		for i := len(res.Bars) - 1; i >= 0; i-- {
			if c := res.Bars[i].Close; !math.IsNaN(c) {
				return c, true, nil
			}
		}
		return 0, false, nil
	}
}

// Rates convierte montos entre monedas usando USD como pivote, con caché (incluidos los fallos,
// para no repetir llamadas a Yahoo).
type Rates struct {
	price PriceSource
	warn  func(format string, args ...any)

	mu    sync.Mutex
	cache map[[2]string]*float64
}

// NewRates crea el conversor. warn puede ser nil.
func NewRates(price PriceSource, warn func(format string, args ...any)) *Rates {
	if warn == nil {
		warn = func(string, ...any) {}
	}
	return &Rates{price: price, warn: warn, cache: map[[2]string]*float64{}}
}

// Rate es la tasa origen→destino (monto_origen · tasa = monto_destino); ok=false si no se pudo obtener.
func (r *Rates) Rate(ctx context.Context, origen, destino string) (float64, bool) {
	if origen == destino {
		return 1, true
	}
	key := [2]string{origen, destino}
	r.mu.Lock()
	if v, hit := r.cache[key]; hit {
		r.mu.Unlock()
		if v == nil {
			return 0, false
		}
		return *v, true
	}
	r.mu.Unlock()

	tasa, ok := r.compute(ctx, origen, destino)
	r.mu.Lock()
	if ok {
		r.cache[key] = &tasa
	} else {
		r.cache[key] = nil
	}
	r.mu.Unlock()
	return tasa, ok
}

func (r *Rates) compute(ctx context.Context, origen, destino string) (float64, bool) {
	price := func(pair string) (float64, bool, error) {
		p, ok, err := r.price(ctx, pair)
		return p, ok && p != 0, err
	}
	var tasa float64
	var ok bool
	var err error
	switch {
	case origen == "USD":
		tasa, ok, err = price(destino + "=X")
	case destino == "USD":
		var p float64
		if p, ok, err = price(origen + "=X"); ok {
			tasa = 1 / p
		}
	default:
		var o, d float64
		var ok1, ok2 bool
		if o, ok1, err = price(origen + "=X"); err == nil {
			d, ok2, err = price(destino + "=X")
		}
		if ok = ok1 && ok2; ok {
			tasa = (1 / o) * d
		}
	}
	if err != nil { // red, parsing, bloqueo de Yahoo…
		r.warn("No se pudo obtener la tasa %s->%s: %v", origen, destino, err)
		return 0, false
	}
	return tasa, ok
}

// ISO4217 son los códigos vigentes (se excluyen metales, fondos y códigos de prueba).
var ISO4217 = func() map[string]bool {
	m := map[string]bool{}
	for _, c := range strings.Fields(`AED AFN ALL AMD ANG AOA ARS AUD AWG AZN BAM BBD BDT BGN BHD BIF BMD BND BOB BRL BSD BTN BWP BYN
    BZD CAD CDF CHF CLP CNY COP CRC CUP CVE CZK DJF DKK DOP DZD EGP ERN ETB EUR FJD FKP GBP GEL GHS
    GIP GMD GNF GTQ GYD HKD HNL HTG HUF IDR ILS INR IQD IRR ISK JMD JOD JPY KES KGS KHR KMF KPW KRW
    KWD KYD KZT LAK LBP LKR LRD LSL LYD MAD MDL MGA MKD MMK MNT MOP MRU MUR MVR MWK MXN MYR MZN NAD
    NGN NIO NOK NPR NZD OMR PAB PEN PGK PHP PKR PLN PYG QAR RON RSD RUB RWF SAR SBD SCR SDG SEK SGD
    SHP SLE SOS SRD SSP STN SYP SZL THB TJS TMT TND TOP TRY TTD TWD TZS UAH UGX USD UYU UZS VES VND
    VUV WST XAF XCD XOF XPF YER ZAR ZMW ZWL`) {
		m[c] = true
	}
	return m
}()

// Subunit es una moneda cotizada en la subunidad de otra (Londres en peniques, etc.).
type Subunit struct {
	ISO     string
	Divisor float64
}

var subunits = map[string]Subunit{
	"GBp": {"GBP", 100}, "GBX": {"GBP", 100}, "ZAc": {"ZAR", 100}, "ZAC": {"ZAR", 100}, "ILA": {"ILS", 100},
}

// Normalize devuelve (moneda ISO, divisor) de un código de Yahoo; ok=false si no es una moneda válida.
// «GBp» → (GBP, 100): precio en moneda = precio / divisor.
func Normalize(code string) (iso string, divisor float64, ok bool) {
	if code == "" {
		return "", 0, false
	}
	if s, hit := subunits[code]; hit {
		return s.ISO, s.Divisor, true
	}
	if up := strings.ToUpper(code); ISO4217[up] && code == up {
		return up, 1, true
	}
	return "", 0, false
}

// USDPair es el ticker de Yahoo con las unidades de la moneda por 1 USD (USDJPY=X).
func USDPair(moneda string) string {
	iso, _, ok := Normalize(moneda)
	if !ok {
		iso = strings.ToUpper(moneda)
	}
	return "USD" + iso + "=X"
}

// Series es una serie temporal ordenada (fechas con zona; V puede tener NaN).
type Series struct {
	T []time.Time
	V []float64
}

// ToUSD convierte precios en moneda local a USD: precio / fx / divisor.
//
// Reindexado exacto con arrastre hacia adelante (ffill): el tipo de cambio de cada fecha de
// precio es el de ese mismo instante, si existe; si falta, se arrastra el último de las fechas
// anteriores (sin mirar al futuro). Antes del primer FX conocido el resultado es NaN.
func ToUSD(precios, fxLocalPorUSD Series, moneda string) Series {
	_, divisor, ok := Normalize(moneda)
	if !ok {
		divisor = 1
	}
	byInstant := make(map[int64]float64, len(fxLocalPorUSD.T))
	for i, t := range fxLocalPorUSD.T {
		byInstant[t.UnixNano()] = fxLocalPorUSD.V[i]
	}
	out := Series{T: precios.T, V: make([]float64, len(precios.V))}
	last := math.NaN()
	for i, t := range precios.T {
		if v, hit := byInstant[t.UnixNano()]; hit && !math.IsNaN(v) {
			last = v
		}
		out.V[i] = precios.V[i] / last / divisor
	}
	return out
}
