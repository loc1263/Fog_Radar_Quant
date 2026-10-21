package etf

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"

	"fogradar/internal/quant"
)

// Rotación del Top 15 vs. la corrida anterior + diversificación por categoría.

// RotFila es una fila de rotación; los campos que no aplican a una tabla quedan nil.
type RotFila struct {
	Ticker, Nombre, Category string
	Racha                    *int64
	RankActual, RankPrevio   *int64
	Score, DeltaScore        *float64
	DeltaRank                *int64
	ScorePrevio              *float64
	Procedencia, Destino     *string
	Precio, High52, Low52    *float64
}

// Rotacion agrupa las tres tablas.
type Rotacion struct {
	Mantienen, Nuevos, Salidas []RotFila
}

// ArchivoPrevio devuelve el archivo `nombre` de la corrida anterior más reciente que lo tenga ("" si ninguna).
// prev son las carpetas de las corridas anteriores, la más reciente primero.
func ArchivoPrevio(prev []string, nombre string) string {
	for _, d := range prev {
		if p := filepath.Join(d, nombre); fileOK(p) {
			return p
		}
	}
	return ""
}

type runTop struct {
	fecha   string
	tickers map[string]bool
}

func listarRunsTop15(dataDir string, prev []string) []runTop {
	var rutas []string
	for i := len(prev) - 1; i >= 0; i-- { // de la más antigua a la más reciente
		if v := filepath.Join(prev[i], Top15File); fileOK(v) {
			rutas = append(rutas, v)
		}
	}
	if v := filepath.Join(dataDir, Top15File); fileOK(v) {
		rutas = append(rutas, v)
	}
	runs := map[string]map[string]bool{}
	for _, r := range rutas {
		df, err := quant.ReadCSVFile(r)
		if err != nil || !df.Has("Fecha") || df.N == 0 {
			continue
		}
		fecha := df.Col("Fecha").Cell(0)
		set := map[string]bool{}
		tk := df.Col("Ticker")
		for i := 0; i < df.N; i++ {
			set[tk.Cell(i)] = true
		}
		runs[fecha] = set // la última corrida de la misma fecha gana
	}
	out := make([]runTop, 0, len(runs))
	for f, s := range runs {
		out = append(out, runTop{f, s})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].fecha < out[j].fecha })
	return out
}

func calcularRacha(runs []runTop, tickers []string) map[string]int64 {
	out := map[string]int64{}
	for _, t := range tickers {
		var n int64
		for i := len(runs) - 1; i >= 0; i-- {
			if runs[i].tickers[t] {
				n++
			} else {
				break
			}
		}
		out[t] = max(n, 1)
	}
	return out
}

func rankMap(f *quant.Frame) map[string]int64 {
	m := map[string]int64{}
	tk := f.Col("Ticker")
	for i := 0; i < f.N; i++ {
		m[tk.Cell(i)] = int64(i + 1)
	}
	return m
}

func scoreMap(f *quant.Frame) map[string]float64 {
	m := map[string]float64{}
	tk, sc := f.Col("Ticker"), f.Col("Score").Floats()
	for i := 0; i < f.N; i++ {
		m[tk.Cell(i)] = sc[i]
	}
	return m
}

func tickerSet(f *quant.Frame) map[string]bool {
	m := map[string]bool{}
	if f.N == 0 {
		return m
	}
	tk := f.Col("Ticker")
	for i := 0; i < f.N; i++ {
		m[tk.Cell(i)] = true
	}
	return m
}

func rowOf(f *quant.Frame, ticker string) int {
	tk := f.Col("Ticker")
	for i := 0; i < f.N; i++ {
		if tk.Cell(i) == ticker {
			return i
		}
	}
	return -1
}

func optF(f *quant.Frame, col string, i int) *float64 {
	c := f.Col(col)
	if c == nil || c.IsNull(i) {
		return nil
	}
	v := c.At(i)
	if math.IsNaN(v) {
		return nil
	}
	return &v
}

func optI(v int64) *int64 { return &v }

func fnan(m map[string]float64, t string) float64 {
	if v, ok := m[t]; ok {
		return v
	}
	return math.NaN()
}

func nanPtr(v float64) *float64 {
	if math.IsNaN(v) {
		return nil
	}
	return &v
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// GenerarDatosRotacion devuelve las tablas mantienen/nuevos/salidas, o nil si no hay corrida anterior completa
// anterior para comparar (prev: carpetas de las corridas anteriores, la más reciente primero). top15/excl/ranking son
// los CSV de la corrida vigente (dataDir).
func GenerarDatosRotacion(dataDir string, prev []string, top15, excl, ranking *quant.Frame) (*Rotacion, error) {
	prevTop15P := ArchivoPrevio(prev, Top15File)
	prevRankP := ArchivoPrevio(prev, RankingFile)
	prevExclP := ArchivoPrevio(prev, ExcluidosFile)
	if prevTop15P == "" || prevRankP == "" || prevExclP == "" {
		return nil, nil
	}
	prevTop15, err := quant.ReadCSVFile(prevTop15P)
	if err != nil {
		return nil, err
	}
	prevRank, err := quant.ReadCSVFile(prevRankP)
	if err != nil {
		return nil, err
	}
	prevExcl, err := quant.ReadCSVFile(prevExclP)
	if err != nil {
		return nil, err
	}

	rankActualTop, rankPrevTop := rankMap(top15), rankMap(prevTop15)
	rankActualUni, rankPrevUni := rankMap(ranking), rankMap(prevRank)
	scoreActual, scorePrev := scoreMap(ranking), scoreMap(prevRank)

	tActual, tPrev := tickerSet(top15), tickerSet(prevTop15)
	exclPrev, exclActual := tickerSet(prevExcl), tickerSet(excl)
	var actualLista []string
	for i := 0; i < top15.N; i++ {
		actualLista = append(actualLista, top15.Col("Ticker").Cell(i))
	}
	racha := calcularRacha(listarRunsTop15(dataDir, prev), actualLista)

	out := &Rotacion{}
	for _, t := range sortedKeys(tActual) {
		i := rowOf(top15, t)
		base := RotFila{Ticker: t, Nombre: top15.Col("Nombre").Cell(i), Category: top15.Col("Category").Cell(i),
			Precio: optF(top15, "Precio", i), High52: optF(top15, "52W High", i), Low52: optF(top15, "52W Low", i)}
		if tPrev[t] {
			r := base
			r.Racha, r.RankActual, r.RankPrevio = optI(racha[t]), optI(rankActualTop[t]), optI(rankPrevTop[t])
			r.Score = nanPtr(fnan(scoreActual, t))
			r.DeltaScore = nanPtr(fnan(scoreActual, t) - fnan(scorePrev, t))
			r.DeltaRank = optI(rankPrevTop[t] - rankActualTop[t])
			out.Mantienen = append(out.Mantienen, r)
			continue
		}
		var proc string
		switch {
		case exclPrev[t]:
			proc = "Estaba excluido por outlier en la corrida anterior"
		case rankPrevUni[t] != 0:
			proc = fmt.Sprintf("Ya estaba en el ranking depurado anterior, puesto #%d", rankPrevUni[t])
		default:
			proc = "No estaba en el universo depurado ni en outliers de la corrida anterior"
		}
		r := base
		r.RankActual, r.Score, r.Procedencia = optI(rankActualTop[t]), nanPtr(fnan(scoreActual, t)), &proc
		out.Nuevos = append(out.Nuevos, r)
	}
	for _, t := range sortedKeys(tPrev) {
		if tActual[t] {
			continue
		}
		i := rowOf(prevTop15, t)
		cat := prevTop15.Col("Category").Cell(i)
		var dest string
		switch {
		case exclActual[t]:
			dest = "Paso a la tabla de excluidos por outlier en esta corrida"
		case inList(cat, CategoriasExcluirRanking):
			dest = fmt.Sprintf("Categoria \"%s\" excluida del analisis por decision de curacion", cat)
		case rankActualUni[t] != 0:
			dest = fmt.Sprintf("Sigue en el ranking depurado, pero bajo al puesto #%d", rankActualUni[t])
		default:
			dest = "Ya no aparece en el universo depurado vigente"
		}
		out.Salidas = append(out.Salidas, RotFila{Ticker: t, Nombre: prevTop15.Col("Nombre").Cell(i), Category: cat,
			RankPrevio: optI(rankPrevTop[t]), ScorePrevio: nanPtr(fnan(scorePrev, t)), Destino: &dest,
			Precio: optF(prevTop15, "Precio", i), High52: optF(prevTop15, "52W High", i), Low52: optF(prevTop15, "52W Low", i)})
	}
	sort.SliceStable(out.Mantienen, func(i, j int) bool { return *out.Mantienen[i].RankActual < *out.Mantienen[j].RankActual })
	sort.SliceStable(out.Nuevos, func(i, j int) bool { return *out.Nuevos[i].RankActual < *out.Nuevos[j].RankActual })
	sort.SliceStable(out.Salidas, func(i, j int) bool { return *out.Salidas[i].RankPrevio < *out.Salidas[j].RankPrevio })
	return out, nil
}

// DiversifFila es la cuenta de ETFs por categoría dentro del Top 15.
type DiversifFila struct {
	Category string
	Conteo   int64
	Pct      float64
}

// DiversificacionTop15 cuenta el Top 15 por categoría (value_counts: conteo descendente; en empate, orden de
// primera aparición).
func DiversificacionTop15(top15 *quant.Frame) []DiversifFila {
	c := top15.Col("Category")
	counts := map[string]int64{}
	var order []string
	for i := 0; i < top15.N; i++ {
		if c.IsNull(i) {
			continue
		}
		k := c.Cell(i)
		if _, ok := counts[k]; !ok {
			order = append(order, k)
		}
		counts[k]++
	}
	sort.SliceStable(order, func(i, j int) bool { return counts[order[i]] > counts[order[j]] })
	out := make([]DiversifFila, len(order))
	for i, k := range order {
		out[i] = DiversifFila{k, counts[k], float64(counts[k]) / float64(top15.N) * 100}
	}
	return out
}
