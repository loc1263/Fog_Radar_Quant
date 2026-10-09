package etf

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"fogradar/internal/quant"
)

// --- Momentum / Crecimiento ---

const (
	pesoM5D = 0.10
	pesoM1M = 0.20
	pesoM6M = 0.30
	pesoM1Y = 0.40
)

// CategoriasExcluirMomentum: lista más amplia (defensivas) que la del ranking principal.
var CategoriasExcluirMomentum = []string{
	"Ultrashort Bond", "Securitized Bond - Focused", "Short-Term Bond", "Short Government",
	"Short-Term Inflation-Protected Bond", "Muni National Short", "Muni Single State Short", "Money Market-Taxable",
	"Prime Money Market", "Target Maturity", "Muni Target Maturity", "Corporate Target Maturity",
	"Target-Date 2030", "Target-Date 2035", "Target-Date 2040", "Target-Date 2045", "Target-Date 2050",
	"Target-Date 2055", "Target-Date 2060", "Target-Date 2065", "Target-Date 2070+", "Target-Date Retirement",
}

var momentumRequired = []string{Col5D, Col1M, Col6M, Col1Y, ColSharpe, ColTreynor, ColBeta}

// perf6mHistorico lee el etf_perf_data_*.csv MÁS ANTIGUO de hist y devuelve Ticker → Performance 6M
// (primer registro de cada ticker); nil si no hay corridas archivadas.
func perf6mHistorico(histDir string) (map[string]float64, error) {
	files, _ := filepath.Glob(filepath.Join(histDir, PatronPerf))
	if len(files) == 0 {
		return nil, nil
	}
	sort.Strings(files)
	df, err := quant.ReadCSVFile(files[0])
	if err != nil {
		return nil, err
	}
	df = df.Filter(tipoETF(df))
	out := map[string]float64{}
	tk := df.Col("Ticker")
	v := df.Col(Col6M).Floats()
	for i := 0; i < df.N; i++ {
		if _, dup := out[tk.Cell(i)]; !dup {
			out[tk.Cell(i)] = v[i]
		}
	}
	return out, nil
}

// MomentumResult es el resultado del análisis de momentum.
type MomentumResult struct {
	FechaTag string
	Ranking  *quant.Frame
	Top15    *quant.Frame
}

// RunMomentum calcula el score de momentum (55 % momentum ponderado, 25 % calidad, 20 % consistencia) y
// escribe etf_momentum_ranking_completo.csv y etf_momentum_top15_score_comp.csv junto al CSV de entrada.
func RunMomentum(csvPath, histDir string) (*MomentumResult, error) {
	df, err := quant.ReadCSVFile(csvPath)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(csvPath)
	u := df.Filter(tipoETF(df))
	u = u.Filter(categoriaNotIn(u, CategoriasExcluirMomentum))
	for _, c := range momentumRequired {
		if !u.Has(c) {
			return nil, fmt.Errorf("el CSV no tiene la columna '%s'", c)
		}
	}
	u = u.DropNA(momentumRequired).Copy()
	// pd.to_numeric(errors="coerce") sobre las columnas numéricas
	for _, c := range append(append([]string{}, momentumRequired...), "Precio", "AUM", "52W High", "52W Low") {
		if col := u.Col(c); col != nil && col.Kind == quant.Str {
			u.SetFloats(c, col.Floats())
		}
	}
	n := u.N
	m5, m1, m6, my := u.Col(Col5D).Floats(), u.Col(Col1M).Floats(), u.Col(Col6M).Floats(), u.Col(Col1Y).Floats()
	mRaw := make([]float64, n)
	for i := range mRaw {
		mRaw[i] = pesoM5D*m5[i] + pesoM1M*m1[i] + pesoM6M*m6[i] + pesoM1Y*my[i]
	}
	u.SetFloats("M_raw", mRaw)
	mNorm := quant.MinMax0100(mRaw)
	u.SetFloats("M_norm", mNorm)

	tre := u.Col(ColTreynor).Floats()
	for i, v := range tre {
		if math.IsInf(v, 0) {
			tre[i] = quant.NaN
		}
	}
	qLow, qHigh := quant.Quantile(tre, 0.01), quant.Quantile(tre, 0.99)
	treCap := quant.Clip(tre, qLow, qHigh)
	u.SetFloats("Treynor_cap", treCap)
	sharpe := u.Col(ColSharpe).Floats()
	qRaw := make([]float64, n)
	for i := range qRaw {
		qRaw[i] = 0.5*sharpe[i] + 0.5*treCap[i]
	}
	u.SetFloats("Q_raw", qRaw)
	qNorm := quant.MinMax0100(qRaw)
	u.SetFloats("Q_norm", qNorm)

	old, err := perf6mHistorico(histDir)
	if err != nil {
		return nil, err
	}
	consNorm := make([]float64, n)
	if old != nil {
		raw := make([]float64, n)
		tk := u.Col("Ticker")
		for i := 0; i < n; i++ {
			raw[i] = quant.NaN
			if o, ok := old[tk.Cell(i)]; ok {
				raw[i] = m6[i] - o
			}
		}
		u.SetFloats("Consist_raw", raw)
		consNorm = quant.MinMax0100(raw)
		for i, v := range consNorm {
			if math.IsNaN(v) {
				consNorm[i] = 50
			}
		}
	} else {
		for i := range consNorm {
			consNorm[i] = 50
		}
	}
	u.SetFloats("Consist_norm", consNorm)
	score := make([]float64, n)
	for i := range score {
		score[i] = 0.55*mNorm[i] + 0.25*qNorm[i] + 0.20*consNorm[i]
	}
	u.SetFloats("Score_comp", score)
	tag := FechaTag(csvPath)
	fechas := make([]string, n)
	for i := range fechas {
		fechas[i] = tag
	}
	u.SetStrs("Fecha", fechas)

	cols := []string{"Ticker", "Nombre", "Category", Col5D, Col1M, Col6M, Col1Y, ColSharpe, ColTreynor, ColBeta,
		"M_norm", "Q_norm", "Consist_norm", "Score_comp", "Fecha", "Precio", "52W High", "52W Low"}
	ranking := u.Take(quant.OrderDesc(score))
	top := ranking.Head(TopN)
	rOut, tOut := ranking.Select(cols), top.Select(cols)
	if err := writeFrame(filepath.Join(dir, MomentumRanking), rOut); err != nil {
		return nil, err
	}
	if err := writeFrame(filepath.Join(dir, MomentumTop15), tOut); err != nil {
		return nil, err
	}
	return &MomentumResult{FechaTag: tag, Ranking: rOut, Top15: tOut}, nil
}

// --- Ranking por categoría ---

const minNCategoria = 5

// RunCategoria agrupa etf_ranking_completo.csv por Category (N ≥ 5) y rankea por Sharpe promedio.
func RunCategoria(rankingPath string) (*quant.Frame, error) {
	ranking, err := quant.ReadCSVFile(rankingPath)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(rankingPath)
	tag := "SINFECHA"
	if c := ranking.Col("Fecha"); c != nil && ranking.N > 0 {
		tag = strings.ReplaceAll(c.Cell(0), "-", "")
	} else if files, _ := filepath.Glob(filepath.Join(dir, "etf_perf_data_*.csv")); len(files) > 0 {
		sort.Strings(files)
		tag = FechaTag(files[len(files)-1])
	}
	cat := ranking.Filter(categoriaNotIn(ranking, CategoriasExcluirRanking))

	cc := cat.Col("Category")
	counts := map[string]int{}
	for i := 0; i < cat.N; i++ {
		if !cc.IsNull(i) {
			counts[cc.Cell(i)]++
		}
	}
	keep := make([]bool, cat.N)
	for i := range keep {
		keep[i] = !cc.IsNull(i) && counts[cc.Cell(i)] >= minNCategoria
	}
	valid := cat.Filter(keep)

	groups := map[string][]int{}
	var names []string
	vc := valid.Col("Category")
	for i := 0; i < valid.N; i++ {
		k := vc.Cell(i)
		if _, ok := groups[k]; !ok {
			names = append(names, k)
		}
		groups[k] = append(groups[k], i)
	}
	sort.Strings(names) // groupby ordena las claves

	sharpe, tre, beta := valid.Col(ColSharpe).Floats(), valid.Col(ColTreynor).Floats(), valid.Col(ColBeta).Floats()
	p1y, p6m, score := valid.Col(Col1Y).Floats(), valid.Col(Col6M).Floats(), valid.Col("Score").Floats()
	tkr := valid.Col("Ticker")
	pick := func(v []float64, idx []int) []float64 {
		out := make([]float64, len(idx))
		for i, j := range idx {
			out[i] = v[j]
		}
		return out
	}
	type row struct {
		cat                         string
		n                           int64
		sh, tr, be, p1, p6, sc, pos float64
		tickers                     string
	}
	rows := make([]row, len(names))
	for k, nm := range names {
		idx := groups[nm]
		r := row{cat: nm, n: int64(len(idx))}
		r.sh, r.tr, r.be = quant.KahanMean(pick(sharpe, idx)), quant.Median(pick(tre, idx)), quant.KahanMean(pick(beta, idx))
		r.p1, r.p6, r.sc = quant.KahanMean(pick(p1y, idx)), quant.KahanMean(pick(p6m, idx)), quant.KahanMean(pick(score, idx))
		posit := 0
		for _, v := range pick(p1y, idx) {
			if v > 0 {
				posit++
			}
		}
		r.pos = float64(posit) / float64(len(idx)) * 100
		ord := append([]int(nil), idx...)
		sort.SliceStable(ord, func(a, b int) bool { return sharpe[ord[a]] > sharpe[ord[b]] })
		ts := make([]string, len(ord))
		for i, j := range ord {
			ts[i] = tkr.Cell(j)
		}
		r.tickers = strings.Join(ts, ", ")
		for _, p := range []*float64{&r.sh, &r.tr, &r.be, &r.p1, &r.p6, &r.sc, &r.pos} {
			*p = quant.Round(*p, 3) // DataFrame.round: rint(x·10³)/10³
		}
		rows[k] = r
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].sh > rows[j].sh })

	out := quant.NewFrame(len(rows))
	cats, ns := &quant.Column{Name: "Category", Kind: quant.Str}, &quant.Column{Name: "N", Kind: quant.Int}
	fcol := func(n string) *quant.Column { return &quant.Column{Name: n, Kind: quant.Float} }
	cSh, cTr, cBe, cP1, cP6, cSc, cPos := fcol("Sharpe_Promedio"), fcol("Treynor_Mediana"), fcol("Beta_Promedio"),
		fcol("Perf_1Y_Promedio"), fcol("Perf_6M_Promedio"), fcol("Score_Promedio"), fcol("Pct_Positivos_1Y")
	cTk := &quant.Column{Name: "Tickers", Kind: quant.Str}
	cF := &quant.Column{Name: "Fecha", Kind: quant.Str}
	for _, r := range rows {
		cats.S, cats.Null = append(cats.S, r.cat), append(cats.Null, false)
		ns.I = append(ns.I, r.n)
		cSh.F, cTr.F, cBe.F = append(cSh.F, r.sh), append(cTr.F, r.tr), append(cBe.F, r.be)
		cP1.F, cP6.F, cSc.F, cPos.F = append(cP1.F, r.p1), append(cP6.F, r.p6), append(cSc.F, r.sc), append(cPos.F, r.pos)
		cTk.S, cTk.Null = append(cTk.S, r.tickers), append(cTk.Null, false)
		cF.S, cF.Null = append(cF.S, tag), append(cF.Null, false)
	}
	for _, c := range []*quant.Column{cats, ns, cSh, cTr, cBe, cP1, cP6, cSc, cPos, cTk, cF} {
		out.Set(c)
	}
	return out, writeFrame(filepath.Join(dir, CategoriasFile), out)
}

// --- Top 15 Cap. ---

// CategoriasCapBucket: las 9 categorías Morningstar con etiqueta de capitalización explícita.
var CategoriasCapBucket = map[string]string{
	"Large Blend": "Large-cap", "Large Value": "Large-cap", "Large Growth": "Large-cap",
	"Mid-Cap Blend": "Mid-cap", "Mid-Cap Value": "Mid-cap", "Mid-Cap Growth": "Mid-cap",
	"Small Blend": "Small-cap", "Small Value": "Small-cap", "Small Growth": "Small-cap",
}

// BucketOrden de los segmentos.
var BucketOrden = []string{"Large-cap", "Mid-cap", "Small-cap"}

var capExcluir = []string{"Utilities", "Real Estate", "Global Real Estate"}

// BenchmarkCandidatos del componente Alpha (proxy del Russell 3000 dentro del universo de ETFs).
var BenchmarkCandidatos = []string{"IWV", "VTI", "SPY"}

const (
	nTopBucket = 5
	topePeso   = 0.015
)

// capPesos pondera proporcional al score con tope por posición (cap and redistribute).
func capPesos(scores []float64, tope float64) []float64 {
	var total float64
	for _, s := range scores {
		total += s
	}
	pesos := make([]float64, len(scores))
	for i, s := range scores {
		pesos[i] = s / total
	}
	fijo := make([]bool, len(pesos))
	for iter := 0; iter < len(pesos)+5; iter++ {
		any := false
		for i := range pesos {
			if !fijo[i] && pesos[i] > tope+1e-12 {
				fijo[i], pesos[i], any = true, tope, true
			}
		}
		if !any {
			break
		}
		var sumFijo, sumLibres float64
		libres := 0
		for i := range pesos {
			if fijo[i] {
				sumFijo += pesos[i]
			} else {
				sumLibres += pesos[i]
				libres++
			}
		}
		if libres == 0 {
			break
		}
		objetivo := 1.0 - sumFijo
		if sumLibres <= 0 || objetivo <= 0 {
			for i := range pesos {
				if !fijo[i] {
					pesos[i] = 0
				}
			}
			break
		}
		factor := objetivo / sumLibres
		for i := range pesos {
			if !fijo[i] {
				pesos[i] *= factor
			}
		}
	}
	return pesos
}

// CapResult es el resultado del análisis Top 15 Cap.
type CapResult struct {
	FechaTag       string
	Portfolio      *quant.Frame
	ResumenBuckets *quant.Frame
	Benchmark      string
	BenchmarkRet1Y float64
}

// RunCap: estrategia tipo VFMO sobre ETFs por segmento de capitalización (5 mejores por Large/Mid/Small).
func RunCap(csvPath string) (*CapResult, error) {
	crudo, err := quant.ReadCSVFile(csvPath)
	if err != nil {
		return nil, err
	}
	crudo = crudo.Copy()
	for _, c := range []string{Col6M, Col1Y, ColBeta, "AUM", "Volumen", "Precio"} {
		if col := crudo.Col(c); col != nil && col.Kind == quant.Str {
			crudo.SetFloats(c, col.Floats())
		}
	}
	bench, benchRet := "", 0.0
	tk := crudo.Col("Ticker")
	y1 := crudo.Col(Col1Y)
	for _, cand := range BenchmarkCandidatos {
		for i := 0; i < crudo.N; i++ {
			if tk.Cell(i) == cand {
				if !y1.IsNull(i) {
					bench, benchRet = cand, y1.At(i)
				}
				break
			}
		}
		if bench != "" {
			break
		}
	}
	if bench == "" {
		return nil, fmt.Errorf("ninguno de los benchmarks candidatos %v está presente en la corrida vigente -- no se puede calcular el componente Alpha", BenchmarkCandidatos)
	}

	u := crudo.Filter(tipoETF(crudo))
	notBench := make([]bool, u.N)
	for i := range notBench {
		notBench[i] = u.Col("Ticker").Cell(i) != bench
	}
	u = u.Filter(notBench).Copy()
	buckets := make([]string, u.N)
	has := make([]bool, u.N)
	cc := u.Col("Category")
	for i := 0; i < u.N; i++ {
		if !cc.IsNull(i) {
			if b, ok := CategoriasCapBucket[cc.Cell(i)]; ok {
				buckets[i], has[i] = b, true
			}
		}
	}
	u.SetStrs("Cap_Bucket", buckets)
	u = u.Filter(has)
	u = u.Filter(categoriaNotIn(u, capExcluir))
	u = u.DropNA([]string{Col6M, Col1Y, ColBeta, "AUM", "Volumen", "Precio"})
	vol, aum := u.Col("Volumen").Floats(), u.Col("AUM").Floats()
	keep := make([]bool, u.N)
	for i := range keep {
		keep[i] = vol[i] >= LiqVolMin && aum[i] >= LiqAUMMin
	}
	u = u.Filter(keep).Copy()

	n := u.N
	c1, c2, beta := u.Col(Col1Y).Floats(), u.Col(Col6M).Floats(), u.Col(ColBeta).Floats()
	alpha := make([]float64, n)
	for i := range alpha {
		alpha[i] = c1[i] - beta[i]*benchRet
	}
	u.SetFloats("Comp_T12T1_aprox", c1)
	u.SetFloats("Comp_T7T1_aprox", c2)
	u.SetFloats("Alpha_CAPM_aprox", alpha)
	n1, n2, n3 := quant.MinMax0100(c1), quant.MinMax0100(c2), quant.MinMax0100(alpha)
	u.SetFloats("Comp1_norm", n1)
	u.SetFloats("Comp2_norm", n2)
	u.SetFloats("Comp3_norm", n3)
	ms := make([]float64, n)
	for i := range ms {
		ms[i] = (n1[i] + n2[i] + n3[i]) / 3.0
	}
	u.SetFloats("Momentum_Score", ms)

	var parts []*quant.Frame
	type resumen struct {
		bucket         string
		nUni           int64
		aumTot         float64
		nSel           int64
		aumSel, pctCub float64
	}
	var res []resumen
	bcol := u.Col("Cap_Bucket")
	for _, b := range BucketOrden {
		var idx []int
		for i := 0; i < n; i++ {
			if bcol.Cell(i) == b {
				idx = append(idx, i)
			}
		}
		sub := u.Take(idx)
		sub = sub.Take(quant.OrderDesc(sub.Col("Momentum_Score").Floats()))
		aumTot := quant.Sum(sub.Col("AUM").Floats())
		nSel := min(nTopBucket, sub.N)
		sel := sub.Head(nSel)
		parts = append(parts, sel)
		aumSel := quant.Sum(sel.Col("AUM").Floats())
		pct := 0.0
		if aumTot > 0 {
			pct = aumSel / aumTot * 100
		}
		res = append(res, resumen{b, int64(sub.N), aumTot, int64(nSel), aumSel, pct})
	}
	portfolio := concat(parts, u)

	pn := portfolio.N
	pesos := make([]float64, pn)
	if pn > 0 {
		scores := portfolio.Col("Momentum_Score").Floats()
		if float64(pn)*topePeso < 1.0 {
			total := quant.Sum(scores)
			for i := range pesos {
				pesos[i] = scores[i] / total * 100
			}
		} else {
			for i, p := range capPesos(scores, topePeso) {
				pesos[i] = p * 100
			}
		}
	}
	portfolio = portfolio.Copy()
	portfolio.SetFloats("Peso_Portfolio", pesos)
	portfolio = portfolio.Take(quant.OrderDesc(pesos)).Copy()
	tag := FechaTag(csvPath)
	fechas, benchs := make([]string, pn), make([]string, pn)
	for i := range fechas {
		fechas[i], benchs[i] = tag, bench
	}
	portfolio.SetStrs("Fecha", fechas)
	portfolio.SetStrs("Benchmark_Alpha", benchs)

	cols := []string{"Ticker", "Nombre", "Category", "Cap_Bucket", Col6M, Col1Y, ColBeta, "AUM", "Volumen",
		"Comp_T12T1_aprox", "Comp_T7T1_aprox", "Alpha_CAPM_aprox", "Comp1_norm", "Comp2_norm", "Comp3_norm",
		"Momentum_Score", "Peso_Portfolio", "Benchmark_Alpha", "Fecha"}
	pOut := portfolio.Select(cols)

	rf := quant.NewFrame(len(res))
	cB, cNU := &quant.Column{Name: "Cap_Bucket", Kind: quant.Str}, &quant.Column{Name: "N_Universo", Kind: quant.Int}
	cAT, cNS := &quant.Column{Name: "AUM_Total_Bucket", Kind: quant.Float}, &quant.Column{Name: "N_Seleccionados", Kind: quant.Int}
	cAS, cPC := &quant.Column{Name: "AUM_Seleccionado", Kind: quant.Float}, &quant.Column{Name: "Pct_AUM_Cubierto", Kind: quant.Float}
	cF, cBA := &quant.Column{Name: "Fecha", Kind: quant.Str}, &quant.Column{Name: "Benchmark_Alpha", Kind: quant.Str}
	cBR := &quant.Column{Name: "Benchmark_Retorno_1Y", Kind: quant.Float}
	for _, r := range res {
		cB.S, cB.Null = append(cB.S, r.bucket), append(cB.Null, false)
		cNU.I, cNS.I = append(cNU.I, r.nUni), append(cNS.I, r.nSel)
		cAT.F, cAS.F, cPC.F = append(cAT.F, r.aumTot), append(cAS.F, r.aumSel), append(cPC.F, r.pctCub)
		cF.S, cF.Null = append(cF.S, tag), append(cF.Null, false)
		cBA.S, cBA.Null = append(cBA.S, bench), append(cBA.Null, false)
		cBR.F = append(cBR.F, benchRet)
	}
	for _, c := range []*quant.Column{cB, cNU, cAT, cNS, cAS, cPC, cF, cBA, cBR} {
		rf.Set(c)
	}
	dir := filepath.Dir(csvPath)
	if err := writeFrame(filepath.Join(dir, CapPortfolio), pOut); err != nil {
		return nil, err
	}
	if err := writeFrame(filepath.Join(dir, CapBuckets), rf); err != nil {
		return nil, err
	}
	return &CapResult{FechaTag: tag, Portfolio: pOut, ResumenBuckets: rf, Benchmark: bench, BenchmarkRet1Y: benchRet}, nil
}

// concat une marcos con las mismas columnas; sin partes devuelve un marco vacío con la estructura de like.
func concat(parts []*quant.Frame, like *quant.Frame) *quant.Frame {
	if len(parts) == 0 {
		return like.EmptyLike()
	}
	out := parts[0].Copy()
	for _, p := range parts[1:] {
		for j, c := range out.Cols {
			pc := p.Cols[j]
			c.F, c.I = append(c.F, pc.F...), append(c.I, pc.I...)
			c.S, c.Null = append(c.S, pc.S...), append(c.Null, pc.Null...)
		}
		out.N += p.N
	}
	return out
}

// Análisis corre los cuatro análisis sobre el etf_perf_data_*.csv de dir; momentum, categoría y Top 15 Cap. no
// bloquean el resto si fallan (mismo criterio que el original) y quedan en Avisos.
type Analisis struct {
	*RankingResult
	Avisos []string
}

// CorrerAnalisis corre los 4 análisis y escribe los outputs en la misma carpeta que el CSV.
func CorrerAnalisis(dataDir, histDir string, warn func(format string, args ...any)) (*Analisis, error) {
	csv := CSVActual(dataDir)
	if csv == "" {
		return nil, fmt.Errorf("no hay ningún %s en %s", PatronPerf, dataDir)
	}
	rr, err := RunRanking(csv)
	if err != nil {
		return nil, err
	}
	a := &Analisis{RankingResult: rr}
	try := func(nombre string, fn func() error) {
		if err := fn(); err != nil {
			warn("Análisis de %s falló, se omite: %v", nombre, err)
			a.Avisos = append(a.Avisos, nombre)
		}
	}
	try("momentum", func() error { _, err := RunMomentum(csv, histDir); return err })
	try("categorías", func() error { _, err := RunCategoria(filepath.Join(dataDir, RankingFile)); return err })
	try("top 15 cap.", func() error { _, err := RunCap(csv); return err })
	return a, nil
}

// CSVActual devuelve el etf_perf_data_*.csv más reciente (por nombre) de la carpeta, o "".
func CSVActual(dir string) string {
	files, _ := filepath.Glob(filepath.Join(dir, PatronPerf))
	if len(files) == 0 {
		return ""
	}
	sort.Strings(files)
	return files[len(files)-1]
}
