package tend

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"time"

	"fogradar/internal/quant"
)

// Rankings de momentum (mensual y semanal) sobre el universo COMPLETO: retorno acumulado en 4
// ventanas (1, 2, 4, 6, 8, 10 y 12 meses; 1, 2, 3 y 4 semanas) y score = promedio de las ventanas escalado por la
// consistencia (proporción de períodos positivos en la ventana más larga). No es un ranking ajustado por riesgo.

// Frecuencias de momentum. La última ventana es la más larga y fija la consistencia y la historia mínima.
var freqs = map[string]struct {
	unit, positiveCol, periodLabel string
	windows                        []int
	minPeriods                     int
}{
	"mensual": {"m", "pct_meses_positivos_12m", "meses", []int{1, 2, 4, 6, 8, 10, 12}, 13},
	"semanal": {"s", "pct_semanas_positivas_4s", "semanas", []int{1, 2, 3, 4}, 5},
}

// Frecuencias en orden de ejecución.
var Frecuencias = []string{"mensual", "semanal"}

// roundExact redondea a n decimales con redondeo decimal exacto (mitades al par).
func roundExact(x float64, n int) float64 {
	v, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', n, 64), 64)
	return v
}

// resampleMonthly: último valor no faltante de cada mes calendario (ME) por columna, en orden.
func resampleMonthly(p *Prices) (labels []string, cols [][]float64) {
	idx := map[string]int{}
	for _, d := range p.Dates {
		k := d.Format("2006-01")
		if _, ok := idx[k]; !ok {
			idx[k] = len(labels)
			labels = append(labels, k)
		}
	}
	cols = make([][]float64, len(p.Cols))
	for j, c := range p.Cols {
		out := make([]float64, len(labels))
		for i := range out {
			out[i] = quant.NaN
		}
		for i, d := range p.Dates {
			if !math.IsNaN(c[i]) {
				out[idx[d.Format("2006-01")]] = c[i]
			}
		}
		cols[j] = out
	}
	return labels, cols
}

// resampleWeekly: último valor de cada semana (termina en domingo), con filas vacías si faltan semanas.
func resampleWeekly(p *Prices) [][]float64 {
	if len(p.Dates) == 0 {
		return p.Cols
	}
	weekEnd := func(d time.Time) time.Time {
		off := (7 - int(d.Weekday())) % 7 // días hasta el domingo
		return d.AddDate(0, 0, off)
	}
	first, last := weekEnd(p.Dates[0]), weekEnd(p.Dates[len(p.Dates)-1])
	n := int(last.Sub(first).Hours()/24)/7 + 1
	cols := make([][]float64, len(p.Cols))
	for j, c := range p.Cols {
		out := make([]float64, n)
		for i := range out {
			out[i] = quant.NaN
		}
		for i, d := range p.Dates {
			if !math.IsNaN(c[i]) {
				out[int(weekEnd(d).Sub(first).Hours()/24)/7] = c[i]
			}
		}
		cols[j] = out
	}
	return cols
}

// periodReturn: retorno entre el cierre de hace `back` velas y el último disponible; NaN si no hay historia.
func periodReturn(s []float64, back int) float64 {
	if len(s) <= back {
		return quant.NaN
	}
	return s[len(s)-1]/s[len(s)-1-back] - 1
}

// positiveRatio: fracción de velas con retorno positivo dentro de las últimas `back`.
func positiveRatio(s []float64, back int) float64 {
	if len(s) <= back {
		return quant.NaN
	}
	var rets []float64
	for i := 1; i < len(s); i++ {
		rets = append(rets, s[i]/s[i-1]-1)
	}
	if len(rets) > back {
		rets = rets[len(rets)-back:]
	}
	if len(rets) == 0 {
		return quant.NaN
	}
	pos := 0
	for _, r := range rets {
		if r > 0 {
			pos++
		}
	}
	return float64(pos) / float64(len(rets))
}

// momentumScore: promedio de las ventanas escalado por (0.5 + 0.5·consistencia); NaN si falta alguna ventana.
func momentumScore(m []float64, pctPos float64) float64 {
	if len(m) == 0 {
		return quant.NaN
	}
	var sum float64
	for _, c := range m {
		if math.IsNaN(c) {
			return quant.NaN
		}
		sum += c
	}
	avg := sum / float64(len(m))
	cons := 0.5
	if !math.IsNaN(pctPos) {
		cons = pctPos
	}
	return avg * (0.5 + 0.5*cons)
}

// RetornoTable ordena el universo por retorno puro de las últimas `weeks` velas semanales (el más alto primero):
// sin score, sin consistencia y sin ajuste por riesgo. Complementa al momentum, cuyo score favorece la tendencia larga.
func RetornoTable(prices *Prices, weeks int, u *Universe) *quant.Frame {
	cols := resampleWeekly(prices)
	type row struct {
		cells []any
		ret   float64
	}
	var rows []row
	for j, t := range prices.Tickers {
		s := dropNaN(cols[j])
		ret := periodReturn(s, weeks)
		if math.IsNaN(ret) {
			continue
		}
		info, ok := u.Info[t]
		if !ok {
			info = Info{Name: t}
		}
		rows = append(rows, row{[]any{t, info.Name, info.Country, u.SectorOf(t), roundExact(s[len(s)-1], 2), roundExact(ret*100, 2)}, ret})
	}
	if len(rows) == 0 {
		return quant.NewFrame(0)
	}
	vals := make([]float64, len(rows))
	for i, r := range rows {
		vals[i] = r.ret
	}
	ord := quant.OrderDesc(vals)
	f := quant.NewFrame(len(rows))
	rank := make([]int64, len(rows))
	for i := range rank {
		rank[i] = int64(i + 1)
	}
	f.SetInts("rank", rank)
	for k, n := range []string{"ticker", "nombre", "pais", "sector", "precio_actual_usd", "retorno_pct"} {
		col := make([]any, len(rows))
		for i, o := range ord {
			col[i] = rows[o].cells[k]
		}
		f.Set(quant.Cells(n, col))
	}
	return f
}

// MomentumTable calcula el ranking completo (ordenado por score descendente, con la columna «rank»).
func MomentumTable(prices *Prices, frequency string, u *Universe) *quant.Frame {
	fq := freqs[frequency]
	var cols [][]float64
	if frequency == "mensual" {
		_, cols = resampleMonthly(prices)
	} else {
		cols = resampleWeekly(prices)
	}
	type row struct {
		cells []any
		score float64
	}
	var rows []row
	for j, t := range prices.Tickers {
		s := dropNaN(cols[j])
		if len(s) < fq.minPeriods {
			continue
		}
		moms := make([]float64, len(fq.windows))
		for k, w := range fq.windows {
			moms[k] = periodReturn(s, w)
		}
		pctPos := positiveRatio(s, fq.windows[len(fq.windows)-1])
		score := momentumScore(moms, pctPos)
		if math.IsNaN(score) {
			continue
		}
		info, ok := u.Info[t]
		if !ok {
			info = Info{Name: t}
		}
		cells := []any{t, info.Name, info.Country, u.SectorOf(t), roundExact(s[len(s)-1], 2)}
		for _, m := range moms {
			if math.IsNaN(m) {
				cells = append(cells, nil)
			} else {
				cells = append(cells, roundExact(m*100, 2))
			}
		}
		if math.IsNaN(pctPos) {
			cells = append(cells, nil)
		} else {
			cells = append(cells, roundExact(pctPos*100, 1))
		}
		cells = append(cells, roundExact(score*100, 2))
		rows = append(rows, row{cells, roundExact(score*100, 2)})
	}
	names := []string{"ticker", "nombre", "pais", "sector", "precio_actual_usd"}
	for _, w := range fq.windows {
		names = append(names, fmt.Sprintf("momentum_%d%s_pct", w, fq.unit))
	}
	names = append(names, fq.positiveCol, "momentum_score_pct")
	vals := make([]float64, len(rows))
	for i, r := range rows {
		vals[i] = r.score
	}
	ord := quant.OrderDesc(vals)
	f := quant.NewFrame(len(rows))
	rank := make([]int64, len(rows))
	for i := range rank {
		rank[i] = int64(i + 1)
	}
	f.SetInts("rank", rank)
	for k, n := range names {
		col := make([]any, len(rows))
		for i, o := range ord {
			col[i] = rows[o].cells[k]
		}
		f.Set(quant.Cells(n, col))
	}
	if len(rows) == 0 {
		return quant.NewFrame(0)
	}
	return f
}

// RunMomentum escribe top_momentum_{mensual,semanal}<key>.csv por análisis (los TOP_N_MOMENTUM mejores) y
// top_retorno_reciente<key>.csv (los TOP_N_RETORNO con mayor retorno en las últimas RETORNO_WEEKS semanas).
func RunMomentum(prices *Prices, runDir string, u *Universe, p Params, warn func(string, ...any)) error {
	for _, spec := range Analyses(u, p) {
		var cols []string
		for _, t := range prices.Tickers {
			if has(spec.Tickers, t) {
				cols = append(cols, t)
			}
		}
		if len(cols) < spec.MinTickers {
			warn("Solo %d tickers disponibles para '%s' (mínimo requerido: %d); se omite el ranking de momentum.", len(cols), spec.Label, spec.MinTickers)
			continue
		}
		for _, fq := range Frecuencias {
			ranked := MomentumTable(prices.Subset(cols), fq, u)
			path := filepath.Join(runDir, "top_momentum_"+fq+spec.Key+".csv")
			if ranked.N == 0 {
				warn("Ningún ticker (%s) tiene los %d %s de historia mínima requeridos; no se generó ranking de momentum %s.",
					spec.Label, freqs[fq].minPeriods, freqs[fq].periodLabel, fq)
				if err := writeEmptyCSV(path); err != nil {
					return err
				}
				continue
			}
			if err := writeFrameFile(path, ranked.Head(p.TopNMomentum)); err != nil {
				return err
			}
		}
		path := filepath.Join(runDir, "top_retorno_reciente"+spec.Key+".csv")
		best := RetornoTable(prices.Subset(cols), p.RetornoWeeks, u)
		if best.N == 0 {
			warn("Ningún ticker (%s) tiene las %d semanas de historia requeridas; no se generó el ranking por retorno reciente.",
				spec.Label, p.RetornoWeeks+1)
			if err := writeEmptyCSV(path); err != nil {
				return err
			}
			continue
		}
		if err := writeFrameFile(path, best.Head(p.TopNRetorno)); err != nil {
			return err
		}
	}
	return nil
}

// writeEmptyCSV deja un archivo vacío.
func writeEmptyCSV(path string) error { return writeFile(path, []byte("\n")) }
