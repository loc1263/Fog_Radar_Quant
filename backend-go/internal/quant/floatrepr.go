// Package quant es la biblioteca numérica y de datos tabulares compartida por los módulos:
// estadística reproducible bit a bit donde importa (cuantiles, desvío poblacional, ranking),
// lectura/escritura de CSV con formato de floats estable, y optimización de portafolios.
package quant

import (
	"math"
	"strconv"
	"strings"
)

// FloatRepr formatea un float64 en su representación más corta:
// la representación más corta que vuelve al mismo valor, en notación fija para 1e-4 <= |x| < 1e16
// y científica fuera de ese rango ("1e-05", "1.5e+16"); los enteros llevan ".0" ("3.0").
func FloatRepr(f float64) string {
	switch {
	case math.IsNaN(f):
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case f == 0:
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}
	s := strconv.FormatFloat(f, 'e', -1, 64) // d.ddde±XX
	neg := s[0] == '-'
	if neg {
		s = s[1:]
	}
	mant, expStr, _ := strings.Cut(s, "e")
	exp, _ := strconv.Atoi(expStr)
	digits := strings.Replace(mant, ".", "", 1)
	decpt := exp + 1 // posición del punto decimal respecto de digits

	var out string
	if -4 <= exp && exp < 16 {
		switch {
		case decpt <= 0:
			out = "0." + strings.Repeat("0", -decpt) + digits
		case decpt >= len(digits):
			out = digits + strings.Repeat("0", decpt-len(digits)) + ".0"
		default:
			out = digits[:decpt] + "." + digits[decpt:]
		}
	} else {
		out = digits[:1]
		if len(digits) > 1 {
			out += "." + digits[1:]
		}
		sign := "+"
		if exp < 0 {
			sign, exp = "-", -exp
		}
		out += "e" + sign + pad2(exp)
	}
	if neg {
		return "-" + out
	}
	return out
}

func pad2(n int) string {
	s := strconv.Itoa(n)
	if len(s) < 2 {
		return "0" + s
	}
	return s
}

// Round redondea a decimals decimales: rint(x·10^d) / 10^d (mitades al par).
func Round(x float64, decimals int) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	p := math.Pow(10, float64(decimals))
	return math.RoundToEven(x*p) / p
}
