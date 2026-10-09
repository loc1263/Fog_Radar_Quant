// Package etf es el módulo ETFs (/api/etf): screener de Yahoo, cuatro análisis cuantitativos sobre el
// etf_perf_data_<fecha>.csv (ranking con outliers, momentum, categorías, Top 15 Cap.), rotación entre
// corridas, informe PDF e historial. Los CSV de salida son byte a byte los del proyecto original.
package etf

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"fogradar/internal/quant"
)

// Config compartida de los análisis.
const TopN = 15

// PeriodCols son las 4 ventanas de rendimiento, en orden 5D → 1M → 6M → 1Y.
var PeriodCols = []string{"Performance 5D (%)", "Performance 1M (%)", "Performance 6M (%)", "Performance 1Y (%)"}

const (
	Col5D, Col1M, Col6M, Col1Y = "Performance 5D (%)", "Performance 1M (%)", "Performance 6M (%)", "Performance 1Y (%)"
	ColSharpe                  = "Sharpe Ratio"
	ColTreynor                 = "Treynor Ratio"
	ColBeta                    = "Beta"
)

// RiskCols son las métricas de riesgo-retorno del proveedor.
var RiskCols = []string{ColSharpe, ColTreynor, ColBeta}

// Parámetros del ranking principal.
const (
	LiqVolMin  = 10_000
	LiqAUMMin  = 30_000_000
	ZThreshold = 6.0
)

var otherRequired = []string{"52W High", "52W Low", "Precio", "Volumen", "AUM"}

// CategoriasExcluirRanking: excluidas del análisis principal por decisión explícita del usuario (2026-07-24).
var CategoriasExcluirRanking = []string{"Ultrashort Bond", "Securitized Bond - Focused", "Target Maturity"}

type weight struct {
	col string
	w   float64
}

// Pesos del score compuesto.
var weights = []weight{
	{"s_sharpe", 0.25}, {"s_treynor", 0.1175}, {"s_1y", 0.14}, {"s_6m", 0.075}, {"s_consistencia", 0.15},
	{"s_tendencia", 0.10}, {"s_baja_dispersion", 0.05}, {"s_bajo_beta", 0.0675}, {"s_cerca_maximo", 0.05},
}

var simpleWeights = []weight{{"ss_5d", 0.20}, {"ss_1m", 0.20}, {"ss_6m", 0.20}, {"ss_1y", 0.20}, {"ss_bajo_beta", 0.20}}

// HistorialCols son las columnas de historial_corridas.csv.
var HistorialCols = []string{"Fecha", "Universo_Bruto", "Universo_Depurado", "N_Outliers", "Sharpe_Promedio", "Beta_Promedio", "Top1_Ticker", "Top1_Score"}

// Nombres de los archivos de la corrida (los del original).
const (
	Top15File       = "etf_top15_recomendados.csv"
	RankingFile     = "etf_ranking_completo.csv"
	ExcluidosFile   = "etf_excluidos_outliers.csv"
	Top15SimpleFile = "etf_top15_simple.csv"
	MomentumRanking = "etf_momentum_ranking_completo.csv"
	MomentumTop15   = "etf_momentum_top15_score_comp.csv"
	CategoriasFile  = "etf_categorias_ranking.csv"
	CapPortfolio    = "etf_russell3000_portfolio.csv"
	CapBuckets      = "etf_russell3000_resumen_buckets.csv"
	HistorialFile   = "historial_corridas.csv"
	PatronPerf      = "etf_perf_data_*.csv"
)

var fechaRe = regexp.MustCompile(`(\d{8})`)

// FechaTag es la fecha de 8 dígitos del nombre del archivo, o «SINFECHA».
func FechaTag(path string) string {
	if m := fechaRe.FindString(filepath.Base(path)); m != "" {
		return m
	}
	return "SINFECHA"
}

func writeFrame(path string, f *quant.Frame) error {
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := f.WriteCSV(fh); err != nil {
		fh.Close()
		return err
	}
	return fh.Close()
}

func inList(s string, list []string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// categoriaNotIn: filas cuya Category NO está en la lista (un NaN no está en la lista, se conserva).
func categoriaNotIn(f *quant.Frame, list []string) []bool {
	c := f.Col("Category")
	keep := make([]bool, f.N)
	for i := range keep {
		keep[i] = c == nil || c.IsNull(i) || !inList(c.Cell(i), list)
	}
	return keep
}

func tipoETF(f *quant.Frame) []bool {
	c := f.Col("TipoInstrumento")
	keep := make([]bool, f.N)
	for i := range keep {
		keep[i] = c != nil && !c.IsNull(i) && c.Cell(i) == "ETF"
	}
	return keep
}

// madZScore: z robusto 0.6745·(x−mediana)/MAD (0 si MAD = 0).
func madZScore(a []float64) []float64 {
	med := quant.Median(a)
	dev := make([]float64, len(a))
	for i, v := range a {
		dev[i] = math.Abs(v - med)
	}
	mad := quant.Median(dev)
	out := make([]float64, len(a))
	if mad == 0 {
		return out
	}
	for i, v := range a {
		out[i] = 0.6745 * (v - med) / mad
	}
	return out
}

// RankingResult es el resultado del análisis principal.
type RankingResult struct {
	FechaTag         string
	NUniversoBruto   int
	NUniversoDepurad int
	NOutliers        int
	Top15            *quant.Frame
	Ranking          *quant.Frame
	Excluidos        *quant.Frame
	Top15Simple      *quant.Frame
}

func round(x float64, d int) float64 {
	v, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', d, 64), 64)
	return v
}

// RunRanking: ranking multi-factor con detección de outliers (etf_ranking_analysis.run). Escribe en la
// carpeta del CSV los 4 CSV de salida más historial_corridas.csv.
func RunRanking(csvPath string) (*RankingResult, error) {
	df, err := quant.ReadCSVFile(csvPath)
	if err != nil {
		return nil, err
	}
	universo := df.Filter(tipoETF(df))
	nBruto := universo.N
	universo = universo.Filter(categoriaNotIn(universo, CategoriasExcluirRanking))
	required := append(append(append([]string{}, PeriodCols...), RiskCols...), otherRequired...)
	for _, c := range required {
		if !universo.Has(c) {
			return nil, fmt.Errorf("el CSV no tiene la columna '%s'", c)
		}
	}
	universo = universo.DropNA(required)
	vol, aum := universo.Col("Volumen").Floats(), universo.Col("AUM").Floats()
	keep := make([]bool, universo.N)
	for i := range keep {
		keep[i] = vol[i] >= LiqVolMin && aum[i] >= LiqAUMMin
	}
	universo = universo.Filter(keep)

	// z-scores robustos por ventana
	z := make([][]float64, len(PeriodCols))
	universo = universo.Copy()
	for k, col := range PeriodCols {
		z[k] = madZScore(universo.Col(col).Floats())
		universo.SetFloats(col+"_z", z[k])
	}
	outlier := make([]bool, universo.N)
	for i := range outlier {
		for k := range PeriodCols {
			if math.Abs(z[k][i]) > ZThreshold {
				outlier[i] = true
			}
		}
	}
	excluidos := universo.Filter(outlier)
	noOut := make([]bool, len(outlier))
	for i, o := range outlier {
		noOut[i] = !o
	}
	limpio := universo.Filter(noOut).Copy()

	cols := []string{"Ticker", "Nombre", "Category"}
	cols = append(cols, PeriodCols...)
	cols = append(cols, "Sharpe Ratio", "Beta", "AUM", "Volumen", "Ventana_Atipica", "Z_Score")
	var excluidosOut *quant.Frame
	if excluidos.N > 0 {
		ventana := make([]string, excluidos.N)
		zsc := make([]float64, excluidos.N)
		zcols := make([][]float64, len(PeriodCols))
		for k, col := range PeriodCols {
			zcols[k] = excluidos.Col(col + "_z").Floats()
		}
		for i := 0; i < excluidos.N; i++ {
			best, bestAbs := 0, -1.0
			for k := range PeriodCols {
				if a := math.Abs(zcols[k][i]); a > bestAbs { // en empate gana la primera ventana
					best, bestAbs = k, a
				}
			}
			ventana[i] = strings.NewReplacer(" (%)", "", "Performance ", "").Replace(PeriodCols[best])
			zsc[i] = zcols[best][i]
		}
		excluidos = excluidos.Copy()
		excluidos.SetStrs("Ventana_Atipica", ventana)
		excluidos.SetFloats("Z_Score", zsc)
		excluidos = excluidos.Take(quant.OrderDesc(zsc))
		excluidosOut = excluidos.Select(cols)
	} else {
		excluidosOut = quant.NewFrame(0)
		for _, n := range cols {
			if c := excluidos.Col(n); c != nil {
				excluidosOut.Set(&quant.Column{Name: n, Kind: c.Kind})
			} else {
				excluidosOut.Set(&quant.Column{Name: n, Kind: quant.Float})
			}
		}
	}

	n := limpio.N
	precio, hi, lo := limpio.Col("Precio").Floats(), limpio.Col("52W High").Floats(), limpio.Col("52W Low").Floats()
	dist, rango := make([]float64, n), make([]float64, n)
	for i := 0; i < n; i++ {
		dist[i] = (precio[i] - hi[i]) / hi[i] * 100
		rango[i] = (hi[i] - lo[i]) / lo[i] * 100
	}
	limpio.SetFloats("Dist_52W_High_%", dist)
	limpio.SetFloats("Rango_52W_%", rango)

	per := make([][]float64, len(PeriodCols))
	for k, c := range PeriodCols {
		per[k] = limpio.Col(c).Floats()
	}
	posit, disp, pasos := make([]int64, n), make([]float64, n), make([]int64, n)
	for i := 0; i < n; i++ {
		vals := make([]float64, len(PeriodCols))
		for k := range PeriodCols {
			vals[k] = per[k][i]
			if per[k][i] > 0 {
				posit[i]++
			}
		}
		disp[i] = quant.StdSample(vals)
		for k := 0; k+1 < len(vals); k++ {
			if vals[k+1] >= vals[k] {
				pasos[i]++
			}
		}
	}
	limpio.SetInts("Periodos_Positivos", posit)
	limpio.SetFloats("Dispersion_Periodos", disp)
	limpio.SetInts("Pasos_Crecientes", pasos)

	pctRank := func(v []float64, asc bool) []float64 {
		r := quant.PctRank(v, asc)
		for i := range r {
			r[i] *= 100
		}
		return r
	}
	toF := func(v []int64) []float64 {
		out := make([]float64, len(v))
		for i, x := range v {
			out[i] = float64(x)
		}
		return out
	}
	absBeta := make([]float64, n)
	for i, b := range limpio.Col("Beta").Floats() {
		absBeta[i] = math.Abs(b)
	}
	limpio.SetFloats("s_sharpe", pctRank(limpio.Col("Sharpe Ratio").Floats(), true))
	limpio.SetFloats("s_treynor", pctRank(limpio.Col("Treynor Ratio").Floats(), true))
	limpio.SetFloats("s_1y", pctRank(per[3], true))
	limpio.SetFloats("s_6m", pctRank(per[2], true))
	limpio.SetFloats("s_consistencia", pctRank(toF(posit), true))
	limpio.SetFloats("s_tendencia", pctRank(toF(pasos), true))
	limpio.SetFloats("s_baja_dispersion", pctRank(disp, false))
	limpio.SetFloats("s_bajo_beta", pctRank(absBeta, false))
	limpio.SetFloats("s_cerca_maximo", pctRank(dist, true))
	limpio.SetFloats("Score", weightedSum(limpio, weights))

	limpio.SetFloats("ss_5d", pctRank(per[0], true))
	limpio.SetFloats("ss_1m", pctRank(per[1], true))
	limpio.SetFloats("ss_6m", pctRank(per[2], true))
	limpio.SetFloats("ss_1y", pctRank(per[3], true))
	limpio.SetFloats("ss_bajo_beta", pctRank(absBeta, false))
	limpio.SetFloats("Score_Simple", weightedSum(limpio, simpleWeights))

	ranking := limpio.Take(quant.OrderDesc(limpio.Col("Score").Floats()))
	top15 := ranking.Head(TopN).Copy()
	rankSimple := limpio.Take(quant.OrderDesc(limpio.Col("Score_Simple").Floats()))
	top15Simple := rankSimple.Head(TopN).Copy()

	dir := filepath.Dir(csvPath)
	tag := FechaTag(csvPath)
	for _, f := range []*quant.Frame{top15, ranking, top15Simple} {
		fechas := make([]string, f.N)
		for i := range fechas {
			fechas[i] = tag
		}
		f.SetStrs("Fecha", fechas)
	}
	simpleCols := append([]string{"Ticker", "Nombre", "Category"}, PeriodCols...)
	simpleCols = append(simpleCols, "Beta", "ss_5d", "ss_1m", "ss_6m", "ss_1y", "ss_bajo_beta", "Score_Simple", "Fecha")
	top15SimpleOut := top15Simple.Select(simpleCols)

	fechasEx := make([]string, excluidosOut.N)
	for i := range fechasEx {
		fechasEx[i] = tag
	}
	excluidosOut = excluidosOut.Copy()
	excluidosOut.SetStrs("Fecha", fechasEx)

	for name, f := range map[string]*quant.Frame{Top15File: top15, RankingFile: ranking, Top15SimpleFile: top15SimpleOut, ExcluidosFile: excluidosOut} {
		if err := writeFrame(filepath.Join(dir, name), f); err != nil {
			return nil, err
		}
	}
	top1Score := top15.Col("Score").Floats()[0]
	if err := ActualizarHistorial(dir, tag, nBruto, n, excluidos.N, quant.Mean(limpio.Col("Sharpe Ratio").Floats()),
		quant.Mean(limpio.Col("Beta").Floats()), top15.Col("Ticker").Cell(0), top1Score); err != nil {
		return nil, err
	}
	return &RankingResult{FechaTag: tag, NUniversoBruto: nBruto, NUniversoDepurad: n, NOutliers: excluidos.N,
		Top15: top15, Ranking: ranking, Excluidos: excluidosOut, Top15Simple: top15SimpleOut}, nil
}

// weightedSum suma Σ col·w de izquierda a derecha .
func weightedSum(f *quant.Frame, ws []weight) []float64 {
	out := make([]float64, f.N)
	for _, w := range ws {
		v := f.Col(w.col).Floats()
		for i := range out {
			out[i] += v[i] * w.w
		}
	}
	return out
}

// ActualizarHistorial agrega (o reemplaza) la fila de la corrida en historial_corridas.csv, ordenado por fecha.
func ActualizarHistorial(dir, fechaTag string, nBruto, nDepurado, nOutliers int, sharpeProm, betaProm float64, top1Ticker string, top1Score float64) error {
	path := filepath.Join(dir, HistorialFile)
	type fila struct {
		fecha                 string
		bruto, depurado, outl int64
		sharpe, beta, top1    float64
		ticker                string
	}
	nueva := fila{fechaTag, int64(nBruto), int64(nDepurado), int64(nOutliers), round(sharpeProm, 4), round(betaProm, 4),
		round(top1Score, 2), top1Ticker}
	var filas []fila
	if _, err := os.Stat(path); err == nil {
		prev, err := readHistorial(path)
		if err != nil {
			return err
		}
		for i := 0; i < prev.N; i++ {
			if f := prev.Col("Fecha").Cell(i); f != fechaTag {
				filas = append(filas, fila{f, int64(prev.Col("Universo_Bruto").At(i)), int64(prev.Col("Universo_Depurado").At(i)),
					int64(prev.Col("N_Outliers").At(i)), prev.Col("Sharpe_Promedio").At(i), prev.Col("Beta_Promedio").At(i),
					prev.Col("Top1_Score").At(i), prev.Col("Top1_Ticker").Cell(i)})
			}
		}
	}
	filas = append(filas, nueva)
	sort.SliceStable(filas, func(i, j int) bool { return filas[i].fecha < filas[j].fecha })
	f := quant.NewFrame(len(filas))
	col := func(name string, k quant.Kind) *quant.Column { return &quant.Column{Name: name, Kind: k} }
	cF, cB, cD, cO := col("Fecha", quant.Str), col("Universo_Bruto", quant.Int), col("Universo_Depurado", quant.Int), col("N_Outliers", quant.Int)
	cS, cBe, cT, cSc := col("Sharpe_Promedio", quant.Float), col("Beta_Promedio", quant.Float), col("Top1_Ticker", quant.Str), col("Top1_Score", quant.Float)
	for _, r := range filas {
		cF.S, cF.Null = append(cF.S, r.fecha), append(cF.Null, false)
		cB.I, cD.I, cO.I = append(cB.I, r.bruto), append(cD.I, r.depurado), append(cO.I, r.outl)
		cS.F, cBe.F, cSc.F = append(cS.F, r.sharpe), append(cBe.F, r.beta), append(cSc.F, r.top1)
		cT.S, cT.Null = append(cT.S, r.ticker), append(cT.Null, false)
	}
	for _, c := range []*quant.Column{cF, cB, cD, cO, cS, cBe, cT, cSc} {
		f.Set(c)
	}
	return writeFrame(path, f)
}

// readHistorial lee el historial con «Fecha» siempre como texto (dtype={"Fecha": str}).
func readHistorial(path string) (*quant.Frame, error) {
	f, err := quant.ReadCSVFile(path)
	if err != nil {
		return nil, err
	}
	if c := f.Col("Fecha"); c != nil && c.Kind != quant.Str {
		f.SetStrs("Fecha", c.Strs())
	}
	return f, nil
}
