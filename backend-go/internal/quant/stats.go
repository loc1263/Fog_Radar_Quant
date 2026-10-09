package quant

import (
	"math"
	"sort"
)

// NaN es math.NaN(), por brevedad.
var NaN = math.NaN()

// pairwiseSum replica la suma de numpy (suma por pares con 8 acumuladores): con el mismo orden de
// operaciones se obtienen los mismos bits que pandas en medias y varianzas.
func pairwiseSum(a []float64) float64 {
	n := len(a)
	switch {
	case n < 8:
		res := 0.0
		for i := 0; i < n; i++ {
			res += a[i]
		}
		return res
	case n <= 128:
		var r [8]float64
		copy(r[:], a[:8])
		i := 8
		for ; i < n-(n%8); i += 8 {
			for j := 0; j < 8; j++ {
				r[j] += a[i+j]
			}
		}
		res := ((r[0] + r[1]) + (r[2] + r[3])) + ((r[4] + r[5]) + (r[6] + r[7]))
		for ; i < n; i++ {
			res += a[i]
		}
		return res
	default:
		n2 := n / 2
		n2 -= n2 % 8
		return pairwiseSum(a[:n2]) + pairwiseSum(a[n2:])
	}
}

// Sum suma como numpy.
func Sum(a []float64) float64 { return pairwiseSum(a) }

// DropNaN devuelve una copia sin NaN.
func DropNaN(a []float64) []float64 {
	out := make([]float64, 0, len(a))
	for _, v := range a {
		if !math.IsNaN(v) {
			out = append(out, v)
		}
	}
	return out
}

// Mean es la media de los valores presentes (NaN si no hay).
func Mean(a []float64) float64 {
	v := DropNaN(a)
	if len(v) == 0 {
		return NaN
	}
	return pairwiseSum(v) / float64(len(v))
}

// StdPop es el desvío poblacional (ddof=0) de los valores presentes, calculado como pandas
// (nanvar): suma de (x-media)² por pares / n.
func StdPop(a []float64) float64 {
	v := DropNaN(a)
	if len(v) == 0 {
		return NaN
	}
	avg := pairwiseSum(v) / float64(len(v))
	sq := make([]float64, len(v))
	for i, x := range v {
		d := avg - x
		sq[i] = d * d
	}
	return math.Sqrt(pairwiseSum(sq) / float64(len(v)))
}

// Quantile es el cuantil q∈[0,1] con interpolación lineal (numpy/pandas por defecto) de los
// valores presentes; NaN si no hay ninguno.
func Quantile(a []float64, q float64) float64 {
	v := DropNaN(a)
	if len(v) == 0 {
		return NaN
	}
	sort.Float64s(v)
	virtual := float64(len(v)-1) * q
	prev := math.Floor(virtual)
	t := virtual - prev
	lo := v[int(prev)]
	hi := lo
	if int(prev)+1 < len(v) {
		hi = v[int(prev)+1]
	}
	diff := hi - lo
	if t >= 0.5 {
		return hi - diff*(1-t) // numpy _lerp
	}
	return lo + diff*t
}

// Winsorize recorta a los cuantiles [qLo, qHi] (sin efecto con menos de 5 datos). Los NaN se conservan.
func Winsorize(a []float64, qLo, qHi float64) []float64 {
	out := append([]float64(nil), a...)
	if len(DropNaN(a)) < 5 {
		return out
	}
	lo, hi := Quantile(a, qLo), Quantile(a, qHi)
	for i, v := range out {
		switch {
		case math.IsNaN(v):
		case v < lo:
			out[i] = lo
		case v > hi:
			out[i] = hi
		}
	}
	return out
}

// ZScore estandariza con media y desvío poblacional de los valores presentes; todo NaN si hay
// menos de 2 datos o el desvío es 0.
func ZScore(a []float64) []float64 {
	out := make([]float64, len(a))
	valid := DropNaN(a)
	if len(valid) < 2 || StdPop(valid) == 0 {
		for i := range out {
			out[i] = NaN
		}
		return out
	}
	m, s := Mean(valid), StdPop(valid)
	for i, v := range a {
		out[i] = (v - m) / s
	}
	return out
}

// RankMinDesc es el ranking descendente con empates en el mínimo (pandas rank(ascending=False,
// method="min")); los NaN quedan NaN.
func RankMinDesc(a []float64) []float64 {
	out := make([]float64, len(a))
	for i, v := range a {
		if math.IsNaN(v) {
			out[i] = NaN
			continue
		}
		greater := 0
		for _, w := range a {
			if !math.IsNaN(w) && w > v {
				greater++
			}
		}
		out[i] = float64(greater + 1)
	}
	return out
}

// OrderDesc devuelve los índices ordenados de mayor a menor, NaN al final; el orden relativo de
// los empates es el original (estable y documentado; pandas no lo garantiza).
func OrderDesc(a []float64) []int {
	idx := make([]int, len(a))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(x, y int) bool {
		vx, vy := a[idx[x]], a[idx[y]]
		switch {
		case math.IsNaN(vx):
			return false
		case math.IsNaN(vy):
			return true
		}
		return vx > vy
	})
	return idx
}

// Median es la mediana de los valores presentes (promedio de los dos centrales si hay cantidad par).
func Median(a []float64) float64 {
	v := DropNaN(a)
	if len(v) == 0 {
		return NaN
	}
	sort.Float64s(v)
	n := len(v)
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}

// PctRank es rank(pct=True, method="average"): rango promedio (empates) / cantidad de valores presentes.
// ascending=false asigna el rango 1 al mayor. Los NaN quedan NaN.
func PctRank(a []float64, ascending bool) []float64 {
	v := DropNaN(a)
	n := float64(len(v))
	sort.Float64s(v)
	out := make([]float64, len(a))
	for i, x := range a {
		if math.IsNaN(x) {
			out[i] = NaN
			continue
		}
		lo := sort.SearchFloat64s(v, x)                                 // cuántos < x
		hi := sort.Search(len(v), func(k int) bool { return v[k] > x }) // cuántos <= x
		eq := float64(hi - lo)
		var r float64
		if ascending {
			r = float64(lo) + (eq+1)/2
		} else {
			r = float64(len(v)-hi) + (eq+1)/2
		}
		out[i] = r / n
	}
	return out
}

// StdSample es el desvío muestral (ddof=1) de los valores presentes, como pandas (nanvar).
func StdSample(a []float64) float64 {
	v := DropNaN(a)
	if len(v) < 2 {
		return NaN
	}
	avg := pairwiseSum(v) / float64(len(v))
	sq := make([]float64, len(v))
	for i, x := range v {
		d := avg - x
		sq[i] = d * d
	}
	return math.Sqrt(pairwiseSum(sq) / float64(len(v)-1))
}

// KahanMean es la media con suma compensada de Kahan, como groupby().mean() de pandas.
func KahanMean(a []float64) float64 {
	var sum, comp float64
	n := 0
	for _, v := range a {
		if math.IsNaN(v) {
			continue
		}
		y := v - comp
		t := sum + y
		comp = (t - sum) - y
		sum = t
		n++
	}
	if n == 0 {
		return NaN
	}
	return sum / float64(n)
}

// Clip recorta a [lo, hi] (los NaN se conservan).
func Clip(a []float64, lo, hi float64) []float64 {
	out := make([]float64, len(a))
	for i, v := range a {
		switch {
		case math.IsNaN(v):
			out[i] = v
		case v < lo:
			out[i] = lo
		case v > hi:
			out[i] = hi
		default:
			out[i] = v
		}
	}
	return out
}

// MinMax0100 normaliza a 0-100: 100·(x−min)/(max−min); 50 en todos si no hay rango (o todo NaN). ±Inf se tratan como NaN.
func MinMax0100(a []float64) []float64 {
	clean := make([]float64, len(a))
	for i, v := range a {
		if math.IsInf(v, 0) {
			v = NaN
		}
		clean[i] = v
	}
	mn, mx := math.Inf(1), math.Inf(-1)
	for _, v := range clean {
		if !math.IsNaN(v) {
			mn, mx = math.Min(mn, v), math.Max(mx, v)
		}
	}
	out := make([]float64, len(a))
	if math.IsInf(mn, 0) || mx == mn {
		for i := range out {
			out[i] = 50
		}
		return out
	}
	for i, v := range clean {
		out[i] = 100 * (v - mn) / (mx - mn)
	}
	return out
}
