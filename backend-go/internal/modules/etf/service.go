package etf

import (
	"math"
	"path/filepath"
	"strings"
	"time"

	"fogradar/internal/core"
	"fogradar/internal/quant"
)

// Transforma los CSV del módulo en las respuestas de la API (misma selección de archivos y mismos cálculos al
// vuelo que el original: Top 15 Vanguard, diversificación, rotación), con claves snake_case.

func fnum(f *quant.Frame, col string, i int) *float64 {
	c := f.Col(col)
	if c == nil {
		return nil
	}
	v := c.At(i)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}

func fstr(f *quant.Frame, col string, i int) *string {
	c := f.Col(col)
	if c == nil || c.IsNull(i) {
		return nil
	}
	s := c.Cell(i)
	return &s
}

func fint(f *quant.Frame, col string, i int) *int64 {
	v := fnum(f, col, i)
	if v == nil {
		return nil
	}
	n := int64(*v)
	return &n
}

// filaDe arma la fila de la API desde la fila i de un CSV (las columnas que no están quedan en null).
func filaDe(f *quant.Frame, i int) EtfFila {
	t := ""
	if c := f.Col("Ticker"); c != nil && !c.IsNull(i) {
		t = c.Cell(i)
	}
	n := func(col string) *float64 { return fnum(f, col, i) }
	s := func(col string) *string { return fstr(f, col, i) }
	return EtfFila{
		Ticker: t, Nombre: s("Nombre"), Categoria: s("Category"), Proveedor: s("Proveedor"),
		Precio: n("Precio"), Volumen: n("Volumen"), Aum: n("AUM"), ExpenseRatio: n("Expense Ratio"),
		Perf5d: n("Performance 5D (%)"), Perf1m: n("Performance 1M (%)"), Perf6m: n("Performance 6M (%)"),
		PerfYtd: n("Performance YTD (%)"), Perf1y: n("Performance 1Y (%)"), Perf5y: n("Performance 5Y (%)"),
		Max52s: n("52W High"), Min52s: n("52W Low"), Sharpe: n("Sharpe Ratio"), Treynor: n("Treynor Ratio"), Beta: n("Beta"),
		DistMax52sPct: n("Dist_52W_High_%"), Rango52sPct: n("Rango_52W_%"), PeriodosPositivos: n("Periodos_Positivos"),
		DispersionPeriodos: n("Dispersion_Periodos"), PasosCrecientes: n("Pasos_Crecientes"),
		SSharpe: n("s_sharpe"), STreynor: n("s_treynor"), S1y: n("s_1y"), S6m: n("s_6m"), SConsistencia: n("s_consistencia"),
		STendencia: n("s_tendencia"), SBajaDispersion: n("s_baja_dispersion"), SBajoBeta: n("s_bajo_beta"),
		SCercaMaximo: n("s_cerca_maximo"), Score: n("Score"),
		Ss5d: n("ss_5d"), Ss1m: n("ss_1m"), Ss6m: n("ss_6m"), Ss1y: n("ss_1y"), SsBajoBeta: n("ss_bajo_beta"),
		ScoreSimple: n("Score_Simple"), VentanaAtipica: s("Ventana_Atipica"), ZScore: n("Z_Score"),
		MNorm: n("M_norm"), QNorm: n("Q_norm"), ConsistNorm: n("Consist_norm"), ScoreComp: n("Score_comp"),
	}
}

func filasDe(f *quant.Frame) []EtfFila {
	out := []EtfFila{}
	if f == nil {
		return out
	}
	for i := 0; i < f.N; i++ {
		out = append(out, filaDe(f, i))
	}
	return out
}

func fechaStr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

// fechaTabla: la «Fecha» de la primera fila (sin decimales), como fecha.
func fechaTabla(f *quant.Frame) *string {
	if f == nil || f.N == 0 || !f.Has("Fecha") {
		return nil
	}
	return fechaStr(FechaDe(strings.Split(f.Col("Fecha").Cell(0), ".")[0]))
}

func tablaDe(f *quant.Frame) EtfTabla { return EtfTabla{Fecha: fechaTabla(f), Filas: filasDe(f)} }

// Modelo expone los parámetros de las metodologías.
func Modelo() EtfModelo {
	etiquetas := map[string]string{
		"s_sharpe": "Sharpe Ratio", "s_treynor": "Treynor Ratio", "s_1y": "Rendimiento 1Y", "s_6m": "Rendimiento 6M",
		"s_consistencia": "Consistencia (períodos positivos)", "s_tendencia": "Tendencia (pasos crecientes)",
		"s_baja_dispersion": "Baja dispersión entre períodos", "s_bajo_beta": "Bajo |Beta|", "s_cerca_maximo": "Cercanía al máximo 52S",
		"ss_5d": "Rendimiento 5D", "ss_1m": "Rendimiento 1M", "ss_6m": "Rendimiento 6M", "ss_1y": "Rendimiento 1Y", "ss_bajo_beta": "Bajo |Beta|",
	}
	pesos := func(ws []weight) []EtfPeso {
		out := make([]EtfPeso, len(ws))
		for i, w := range ws {
			out[i] = EtfPeso{Clave: w.col, Etiqueta: etiquetas[w.col], Peso: w.w}
		}
		return out
	}
	return EtfModelo{
		TopN: TopN, Pesos: pesos(weights), PesosSimple: pesos(simpleWeights),
		PesosMomentum: []EtfPeso{
			{"m_norm", "Momentum (M)", 0.55}, {"q_norm", "Calidad Sharpe/Treynor (Q)", 0.25},
			{"consist_norm", "Consistencia 6M vs. corrida más antigua", 0.20}},
		PesosMomentumM: []EtfPeso{{"perf_5d", "5D", pesoM5D}, {"perf_1m", "1M", pesoM1M}, {"perf_6m", "6M", pesoM6M}, {"perf_1y", "1Y", pesoM1Y}},
		ZThreshold:     ZThreshold, LiqVolMin: LiqVolMin, LiqAumMin: LiqAUMMin,
		CategoriasExcluidas: append([]string{}, CategoriasExcluirRanking...), CategoriasExcluidasMomentum: append([]string{}, CategoriasExcluirMomentum...),
		MinNCategoria: minNCategoria, CapNPorBucket: nTopBucket, CapTopePesoPct: topePeso * 100,
		CapBenchmarks: append([]string{}, BenchmarkCandidatos...),
	}
}

func historialFila(f *quant.Frame, i int) EtfHistorialFila {
	h := EtfHistorialFila{SharpePromedio: fnum(f, "Sharpe_Promedio", i), BetaPromedio: fnum(f, "Beta_Promedio", i),
		Top1Ticker: fstr(f, "Top1_Ticker", i), Top1Score: fnum(f, "Top1_Score", i)}
	if t := FechaDe(f.Col("Fecha").Cell(i)); t != nil {
		h.Fecha = t.Format("2006-01-02")
	}
	if v := fint(f, "Universo_Bruto", i); v != nil {
		h.UniversoBruto = *v
	}
	if v := fint(f, "Universo_Depurado", i); v != nil {
		h.UniversoDepurado = *v
	}
	if v := fint(f, "N_Outliers", i); v != nil {
		h.NOutliers = *v
	}
	return h
}

// sortedByFecha devuelve el marco ordenado por «Fecha» ascendente (estable).
func sortedByFecha(f *quant.Frame) *quant.Frame {
	vals := make([]float64, f.N)
	for i := range vals {
		vals[i] = -f.Col("Fecha").At(i) // OrderDesc de −fecha = fecha ascendente
	}
	return f.Take(quant.OrderDesc(vals))
}

// Resumen es el resumen de la corrida vigente + parámetros del modelo.
func Resumen(s *core.Settings) (*EtfResumen, *core.APIError) {
	unlock, e := BloqueoDatos(s, 0)
	if e != nil {
		return nil, e
	}
	csv := CSVActual(CurrentDir(s))
	if csv == "" {
		unlock()
		return nil, core.NoData(SinCorrida)
	}
	hist, e := Leer(s, HistorialFile)
	generado := GeneradoEn(s, Top15File)
	unlock()
	if e != nil {
		return nil, e
	}
	out := &EtfResumen{Fecha: fechaStr(FechaDe(filepath.Base(csv))), ArchivoCsv: filepath.Base(csv), Modelo: Modelo(),
		TieneCorridaAnterior: HayCorridaAnterior(s)}
	if generado != nil {
		out.GeneradoEn = &core.Fecha{Time: *generado}
	}
	if hist != nil && hist.N > 0 {
		sorted := sortedByFecha(hist)
		out.Resumen = core.Some(historialFila(sorted, sorted.N-1))
	}
	return out, nil
}

// Tabla sirve un CSV de la corrida vigente como tabla.
func Tabla(s *core.Settings, nombre, detalle string) (*EtfTabla, *core.APIError) {
	unlock, e := BloqueoDatos(s, 0)
	if e != nil {
		return nil, e
	}
	df, e := LeerRequerido(s, nombre, detalle)
	unlock()
	if e != nil {
		return nil, e
	}
	t := tablaDe(df)
	return &t, nil
}

// Top15Vanguard: el ranking principal filtrado a Proveedor == «Vanguard» antes de cortar al Top N.
func Top15Vanguard(s *core.Settings) (*EtfTabla, *core.APIError) {
	unlock, e := BloqueoDatos(s, 0)
	if e != nil {
		return nil, e
	}
	df, e := LeerRequerido(s, RankingFile, "")
	unlock()
	if e != nil {
		return nil, e
	}
	keep := make([]bool, df.N)
	if p := df.Col("Proveedor"); p != nil {
		for i := range keep {
			keep[i] = !p.IsNull(i) && p.Cell(i) == "Vanguard"
		}
	}
	v := df.Filter(keep)
	v = v.Take(quant.OrderDesc(v.Col("Score").Floats())).Head(TopN)
	return &EtfTabla{Fecha: fechaTabla(df), Filas: filasDe(v)}, nil
}

// Diversificacion del Top 15 por categoría.
func Diversificacion(s *core.Settings) (*EtfDiversificacion, *core.APIError) {
	unlock, e := BloqueoDatos(s, 0)
	if e != nil {
		return nil, e
	}
	top15, e := LeerRequerido(s, Top15File, "")
	unlock()
	if e != nil {
		return nil, e
	}
	out := &EtfDiversificacion{Fecha: fechaTabla(top15), Filas: []EtfDiversificacionFila{}}
	for _, d := range DiversificacionTop15(top15) {
		out.Filas = append(out.Filas, EtfDiversificacionFila{Categoria: d.Category, Conteo: d.Conteo, Pct: d.Pct})
	}
	return out, nil
}

func rotFila(r RotFila) EtfRotacionFila {
	n, c := r.Nombre, r.Category
	return EtfRotacionFila{
		EtfFila: EtfFila{Ticker: r.Ticker, Nombre: &n, Categoria: &c, Precio: r.Precio, Max52s: r.High52, Min52s: r.Low52, Score: r.Score},
		Racha:   r.Racha, RankActual: r.RankActual, RankPrevio: r.RankPrevio, DeltaScore: r.DeltaScore, DeltaRank: r.DeltaRank,
		ScorePrevio: r.ScorePrevio, Procedencia: r.Procedencia, Destino: r.Destino,
	}
}

func rotFilas(rs []RotFila) []EtfRotacionFila {
	out := []EtfRotacionFila{}
	for _, r := range rs {
		out = append(out, rotFila(r))
	}
	return out
}

// RotacionVsAnterior: rotación del Top 15 vs. la corrida archivada anterior.
func RotacionVsAnterior(s *core.Settings) (*EtfRotacion, *core.APIError) {
	unlock, e := BloqueoDatos(s, 0)
	if e != nil {
		return nil, e
	}
	top15, e1 := LeerRequerido(s, Top15File, "")
	ranking, e2 := LeerRequerido(s, RankingFile, "")
	excl, e3 := LeerRequerido(s, ExcluidosFile, "")
	var rot *Rotacion
	var err error
	var previo string
	if e1 == nil && e2 == nil && e3 == nil {
		rot, err = GenerarDatosRotacion(CurrentDir(s), HistDir(s), top15, excl, ranking)
		previo = EncontrarArchivoPrevio(HistDir(s), Top15File)
	}
	unlock()
	for _, x := range []*core.APIError{e1, e2, e3} {
		if x != nil {
			return nil, x
		}
	}
	if err != nil {
		return nil, core.Err(503, "invalid_data", err.Error())
	}
	fecha := fechaTabla(top15)
	if rot == nil {
		return &EtfRotacion{Disponible: false, Fecha: fecha, Mantienen: []EtfRotacionFila{}, Nuevos: []EtfRotacionFila{}, Salidas: []EtfRotacionFila{}}, nil
	}
	out := &EtfRotacion{Disponible: true, Fecha: fecha, Mantienen: rotFilas(rot.Mantienen), Nuevos: rotFilas(rot.Nuevos), Salidas: rotFilas(rot.Salidas)}
	if previo != "" {
		out.FechaPrevia = fechaStr(FechaDe(filepath.Base(previo)))
	}
	return out, nil
}

// Categorias: ranking por categoría.
func Categorias(s *core.Settings) (*EtfCategorias, *core.APIError) {
	unlock, e := BloqueoDatos(s, 0)
	if e != nil {
		return nil, e
	}
	df, e := LeerRequerido(s, CategoriasFile, "El análisis de Categoría no se corrió en esta corrida.")
	unlock()
	if e != nil {
		return nil, e
	}
	out := &EtfCategorias{Fecha: fechaTabla(df), MinNCategoria: minNCategoria, Filas: []EtfCategoriaFila{}}
	for i := 0; i < df.N; i++ {
		f := EtfCategoriaFila{N: 0, Tickers: []string{}}
		if c := fstr(df, "Category", i); c != nil {
			f.Categoria = *c
		}
		if n := fint(df, "N", i); n != nil {
			f.N = *n
		}
		f.SharpePromedio, f.TreynorMediana, f.BetaPromedio = fnum(df, "Sharpe_Promedio", i), fnum(df, "Treynor_Mediana", i), fnum(df, "Beta_Promedio", i)
		f.Perf1yPromedio, f.Perf6mPromedio = fnum(df, "Perf_1Y_Promedio", i), fnum(df, "Perf_6M_Promedio", i)
		f.ScorePromedio, f.PctPositivos1y = fnum(df, "Score_Promedio", i), fnum(df, "Pct_Positivos_1Y", i)
		if tk := fstr(df, "Tickers", i); tk != nil {
			for _, t := range strings.Split(*tk, ",") {
				if t = strings.TrimSpace(t); t != "" {
					f.Tickers = append(f.Tickers, t)
				}
			}
		}
		out.Filas = append(out.Filas, f)
	}
	return out, nil
}

// Top15Cap: Top 15 Cap. (momentum factor tipo VFMO).
func Top15Cap(s *core.Settings) (*EtfCap, *core.APIError) {
	unlock, e := BloqueoDatos(s, 0)
	if e != nil {
		return nil, e
	}
	pf, e := LeerRequerido(s, CapPortfolio, "El análisis Top 15 Cap. no se corrió en esta corrida.")
	var buckets *quant.Frame
	if e == nil {
		buckets, e = Leer(s, CapBuckets)
	}
	unlock()
	if e != nil {
		return nil, e
	}
	out := &EtfCap{Fecha: fechaTabla(pf), Filas: []EtfCapFila{}, Buckets: []EtfCapBucket{}}
	if buckets != nil && buckets.N > 0 {
		out.BenchmarkTicker = fstr(buckets, "Benchmark_Alpha", 0)
		out.BenchmarkRetorno1y = fnum(buckets, "Benchmark_Retorno_1Y", 0)
	} else if pf.Has("Benchmark_Alpha") && pf.N > 0 {
		out.BenchmarkTicker = fstr(pf, "Benchmark_Alpha", 0)
	}
	for i := 0; i < pf.N; i++ {
		out.Filas = append(out.Filas, EtfCapFila{EtfFila: filaDe(pf, i), CapBucket: fstr(pf, "Cap_Bucket", i),
			CompT12t1Aprox: fnum(pf, "Comp_T12T1_aprox", i), CompT7t1Aprox: fnum(pf, "Comp_T7T1_aprox", i),
			AlphaCapmAprox: fnum(pf, "Alpha_CAPM_aprox", i), Comp1Norm: fnum(pf, "Comp1_norm", i), Comp2Norm: fnum(pf, "Comp2_norm", i),
			Comp3Norm: fnum(pf, "Comp3_norm", i), MomentumScore: fnum(pf, "Momentum_Score", i), PesoPortfolio: fnum(pf, "Peso_Portfolio", i)})
	}
	if buckets != nil {
		for i := 0; i < buckets.N; i++ {
			b := EtfCapBucket{AumTotalBucket: fnum(buckets, "AUM_Total_Bucket", i), AumSeleccionado: fnum(buckets, "AUM_Seleccionado", i),
				PctAumCubierto: fnum(buckets, "Pct_AUM_Cubierto", i)}
			if c := fstr(buckets, "Cap_Bucket", i); c != nil {
				b.CapBucket = *c
			}
			if n := fint(buckets, "N_Universo", i); n != nil {
				b.NUniverso = *n
			}
			if n := fint(buckets, "N_Seleccionados", i); n != nil {
				b.NSeleccionados = *n
			}
			out.Buckets = append(out.Buckets, b)
		}
	}
	return out, nil
}
