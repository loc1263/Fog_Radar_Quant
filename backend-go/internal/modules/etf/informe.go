package etf

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"fogradar/internal/pdf"
	"fogradar/internal/quant"
)

// Informe PDF ejecutivo del módulo ETF: mismas secciones y columnas (Top 15, Excluidos por distorsión,
// Rotación, Diversificación, Ranking por categoría, Top 15 Momentum, Top 15 Perf. + Beta y Top 15 Cap.). Apaisado.

// ErrSinCorrida es el error cuando no hay corrida procesada.
var ErrSinCorrida = errors.New("Todavía no hay ninguna corrida procesada.")

func fv(f *quant.Frame, col string, i int) *float64 {
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

func sv(f *quant.Frame, col string, i int, def string) string {
	c := f.Col(col)
	if c == nil || c.IsNull(i) {
		return def
	}
	return c.Cell(i)
}

func pnum(f *quant.Frame, col string, i, dec int) pdf.Cell {
	return pdf.Cell{Text: pdf.Num(fv(f, col, i), dec)}
}
func ppct(f *quant.Frame, col string, i, dec int) pdf.Cell {
	return pdf.Cell{Text: pdf.Pct(fv(f, col, i), dec)}
}
func ptxt(t string) pdf.Cell    { return pdf.Cell{Text: t} }
func pticker(t string) pdf.Cell { return pdf.Cell{Text: t, Bold: true, Color: pdf.Navy} }

// nombreCat es «Nombre» en negrita con la categoría debajo (9 pt navy).
func nombreCat(f *quant.Frame, i int) pdf.Cell {
	return pdf.Cell{Text: sv(f, "Nombre", i, ""), Sub: sv(f, "Category", i, ""), Bold: true}
}

func tablaTop15(df *quant.Frame, colScore string) ([]string, []string, []float64, [][]pdf.Cell) {
	rows := make([][]pdf.Cell, df.N)
	for i := 0; i < df.N; i++ {
		rows[i] = []pdf.Cell{ptxt(fmt.Sprint(i + 1)), pticker(sv(df, "Ticker", i, "")), nombreCat(df, i),
			ppct(df, "Performance 5D (%)", i, 2), ppct(df, "Performance 1M (%)", i, 2), ppct(df, "Performance 6M (%)", i, 2),
			ppct(df, "Performance 1Y (%)", i, 2), pnum(df, "Sharpe Ratio", i, 2), pnum(df, "Beta", i, 2),
			pdf.Acento(pdf.Num(fv(df, colScore, i), 1))}
	}
	return []string{"#", "Ticker", "Nombre / Categoría", "5D", "1M", "6M", "1Y", "Sharpe", "Beta", "Score"},
		[]string{"r", "l", "l", "r", "r", "r", "r", "r", "r", "r"},
		[]float64{0.03, 0.07, 0.32, 0.08, 0.08, 0.08, 0.08, 0.08, 0.07, 0.11}, rows
}

func vacio(f *quant.Frame) bool { return f == nil || f.N == 0 }

func leerCSV(dir, nombre string) *quant.Frame {
	p := filepath.Join(dir, nombre)
	if _, err := os.Stat(p); err != nil {
		return nil
	}
	f, err := quant.ReadCSVFile(p)
	if err != nil {
		return nil
	}
	return f
}

func pint(v *int64) *float64 {
	if v == nil {
		return nil
	}
	f := float64(*v)
	return &f
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// GenerarPDF arma el informe a partir de la última corrida en dataDir. Devuelve (pdf, fecha «dd/mm/aaaa»).
func GenerarPDF(dataDir, histDir string) ([]byte, string, error) {
	top15 := leerCSV(dataDir, Top15File)
	if vacio(top15) {
		return nil, "", ErrSinCorrida
	}
	excluidos := leerCSV(dataDir, ExcluidosFile)
	ranking := leerCSV(dataDir, RankingFile)
	categorias := leerCSV(dataDir, CategoriasFile)
	var rot *Rotacion
	if excluidos != nil && ranking != nil {
		r, err := GenerarDatosRotacion(dataDir, histDir, top15, excluidos, ranking)
		if err != nil {
			return nil, "", err
		}
		rot = r
	}
	fecha := pdf.Vacio
	if c := top15.Col("Fecha"); c != nil && top15.N > 0 {
		fecha = pdf.FechaYYYYMMDD(c.Cell(0))
	}

	d, err := pdf.New("ETFs — Informe Ejecutivo", "Corrida: "+fecha, "Fog Radar Quant - ETFs - Informe Ejecutivo")
	if err != nil {
		return nil, "", err
	}

	d.Seccion("TOP 15 RECOMENDADOS")
	h, a, fr, rows := tablaTop15(top15, "Score")
	d.Tabla(h, a, fr, rows)
	d.Spacer(14)

	d.Seccion("EXCLUIDOS POR DISTORSIÓN ESTADÍSTICA")
	if !vacio(excluidos) {
		d.Nota("La exclusión responde a una distorsión estadística frente a un universo diversificado (|z-score MAD| > 6) y no implica necesariamente mala calidad del fondo.")
		d.Spacer(4)
		rows := make([][]pdf.Cell, excluidos.N)
		for i := 0; i < excluidos.N; i++ {
			rows[i] = []pdf.Cell{pticker(sv(excluidos, "Ticker", i, "")), nombreCat(excluidos, i),
				ppct(excluidos, "Performance 5D (%)", i, 2), ppct(excluidos, "Performance 1M (%)", i, 2),
				ppct(excluidos, "Performance 6M (%)", i, 2), ppct(excluidos, "Performance 1Y (%)", i, 2),
				pnum(excluidos, "Sharpe Ratio", i, 2), pnum(excluidos, "Beta", i, 2),
				ptxt(sv(excluidos, "Ventana_Atipica", i, pdf.Vacio)), pdf.Signo(fv(excluidos, "Z_Score", i), 2)}
		}
		d.Tabla([]string{"Ticker", "Nombre / Categoría", "5D", "1M", "6M", "1Y", "Sharpe", "Beta", "Ventana atípica", "Z-score"},
			[]string{"l", "l", "r", "r", "r", "r", "r", "r", "l", "r"},
			[]float64{0.07, 0.28, 0.07, 0.07, 0.07, 0.07, 0.07, 0.06, 0.12, 0.12}, rows)
	} else {
		d.Nota("Ningún ETF fue excluido por distorsión en esta corrida.")
	}
	d.PageBreak()

	d.Seccion("ROTACIÓN VS. CORRIDA ANTERIOR")
	if rot == nil {
		d.Nota("No hay todavía una corrida archivada para comparar. Esta sección se completa automáticamente en la próxima corrida.")
	} else {
		d.Badge(fmt.Sprintf("Se mantienen en el Top 15 (%d)", len(rot.Mantienen)), pdf.Verde)
		if len(rot.Mantienen) > 0 {
			rows := make([][]pdf.Cell, len(rot.Mantienen))
			for i, r := range rot.Mantienen {
				rows[i] = []pdf.Cell{pticker(r.Ticker), {Text: r.Nombre, Sub: r.Category, Bold: true},
					ptxt(pdf.Num(pint(r.RankActual), 0)), ptxt(pdf.Num(pint(r.RankPrevio), 0)), ptxt(pdf.Num(pint(r.Racha), 0)),
					ptxt(pdf.Num(r.Score, 1)), pdf.Signo(r.DeltaScore, 1), pdf.Signo(pint(r.DeltaRank), 0)}
			}
			d.Tabla([]string{"Ticker", "Nombre / Categoría", "# Act.", "# Prev.", "Racha", "Score", "ΔScore", "ΔRank"},
				[]string{"l", "l", "r", "r", "r", "r", "r", "r"}, []float64{0.08, 0.34, 0.09, 0.09, 0.09, 0.10, 0.10, 0.11}, rows)
		} else {
			d.Nota("Ningún ETF se mantuvo (rotación total).")
		}

		d.Badge(fmt.Sprintf("Nuevos ingresos (%d)", len(rot.Nuevos)), pdf.Azul)
		if len(rot.Nuevos) > 0 {
			d.Tabla(rotHeaders("# Act."), rotAligns, rotFracs, rotRows(rot.Nuevos, false))
		} else {
			d.Nota("No hubo nuevos ingresos en esta corrida.")
		}

		d.Badge(fmt.Sprintf("Salidas del Top 15 (%d)", len(rot.Salidas)), pdf.Rojo)
		if len(rot.Salidas) > 0 {
			d.Tabla(rotHeaders("# Prev."), rotAligns, rotFracs, rotRows(rot.Salidas, true))
		} else {
			d.Nota("No hubo salidas en esta corrida.")
		}
	}
	d.PageBreak()

	d.Seccion("DIVERSIFICACIÓN DEL TOP 15")
	div := DiversificacionTop15(top15)
	drows := make([][]pdf.Cell, len(div))
	for i, r := range div {
		c := float64(r.Conteo)
		drows[i] = []pdf.Cell{ptxt(r.Category), ptxt(pdf.Num(&c, 0)), {Text: pdf.Num(&r.Pct, 1) + "%", Bold: true}}
	}
	d.Tabla([]string{"Categoría", "N ETFs", "% del Top 15"}, []string{"l", "r", "r"}, []float64{0.6, 0.2, 0.2}, drows)
	d.Spacer(14)

	d.Seccion("RANKING POR CATEGORÍA")
	if !vacio(categorias) {
		d.Nota("Rankea categorías completas (no ETFs individuales) dentro del universo ya depurado de distorsiones.")
		d.Spacer(4)
		cats := categorias.Head(15)
		rows := make([][]pdf.Cell, cats.N)
		for i := 0; i < cats.N; i++ {
			pos := fv(cats, "Pct_Positivos_1Y", i)
			rows[i] = []pdf.Cell{ptxt(fmt.Sprint(i + 1)), ptxt(sv(cats, "Category", i, "")), pnum(cats, "N", i, 0),
				pnum(cats, "Sharpe_Promedio", i, 2), pnum(cats, "Treynor_Mediana", i, 2), pnum(cats, "Beta_Promedio", i, 2),
				ppct(cats, "Perf_1Y_Promedio", i, 1), ppct(cats, "Perf_6M_Promedio", i, 1),
				pdf.Acento(pdf.Num(fv(cats, "Score_Promedio", i), 1)), ptxt(pdf.Num(pos, 0) + "%")}
		}
		d.Tabla([]string{"#", "Categoría", "N", "Sharpe Prom.", "Treynor Med.", "Beta Prom.", "Perf. 1Y Prom.", "Perf. 6M Prom.", "Score Prom.", "% Pos. 1Y"},
			[]string{"r", "l", "r", "r", "r", "r", "r", "r", "r", "r"}, []float64{0.03, 0.27, 0.06, 0.09, 0.09, 0.08, 0.09, 0.09, 0.09, 0.11}, rows)
	} else {
		d.Nota("El análisis de Categoría no se corrió en esta corrida.")
	}
	d.PageBreak()

	d.Seccion("TOP 15 MOMENTUM / CRECIMIENTO")
	if mom := leerCSV(dataDir, MomentumTop15); !vacio(mom) {
		d.Nota("Análisis complementario al ranking principal: en vez de minimizar riesgo y descartar distorsiones, busca momentum de rendimiento.")
		d.Spacer(4)
		h, a, fr, rows := tablaTop15(mom, "Score_comp")
		d.Tabla(h, a, fr, rows)
	} else {
		d.Nota("El análisis de Momentum/Crecimiento no se corrió en esta corrida.")
	}
	d.PageBreak()

	d.Seccion("TOP 15 (PERFORMANCE + BETA)")
	if simple := leerCSV(dataDir, Top15SimpleFile); !vacio(simple) {
		d.Nota("Ranking considerando únicamente el rendimiento por período (5D, 1M, 6M, 1Y) y el Beta.")
		d.Spacer(4)
		rows := make([][]pdf.Cell, simple.N)
		for i := 0; i < simple.N; i++ {
			rows[i] = []pdf.Cell{ptxt(fmt.Sprint(i + 1)), pticker(sv(simple, "Ticker", i, "")), nombreCat(simple, i),
				ppct(simple, "Performance 5D (%)", i, 2), ppct(simple, "Performance 1M (%)", i, 2),
				ppct(simple, "Performance 6M (%)", i, 2), ppct(simple, "Performance 1Y (%)", i, 2),
				pnum(simple, "Beta", i, 2), pdf.Acento(pdf.Num(fv(simple, "Score_Simple", i), 1))}
		}
		d.Tabla([]string{"#", "Ticker", "Nombre / Categoría", "5D", "1M", "6M", "1Y", "Beta", "Score"},
			[]string{"r", "l", "l", "r", "r", "r", "r", "r", "r"}, []float64{0.03, 0.08, 0.35, 0.09, 0.09, 0.09, 0.09, 0.08, 0.10}, rows)
	} else {
		d.Nota("Este ranking alternativo no se corrió en esta corrida.")
	}
	d.PageBreak()

	d.Seccion("TOP 15 CAP.")
	if cap := leerCSV(dataDir, CapPortfolio); !vacio(cap) {
		d.Nota("Estrategia tipo Vanguard U.S. Momentum Factor ETF (VFMO), aplicada sobre ETFs en vez de acciones individuales del Russell 3000. " +
			"Los 5 ETFs de mayor Score de Momentum de cada bucket de capitalización (Large/Mid/Small), 15 en total, ponderados proporcional al Score.")
		d.Spacer(4)
		if res := leerCSV(dataDir, CapBuckets); !vacio(res) {
			rows := make([][]pdf.Cell, res.N)
			for i := 0; i < res.N; i++ {
				rows[i] = []pdf.Cell{ptxt(sv(res, "Cap_Bucket", i, "")),
					ptxt(pdf.Num(fv(res, "N_Seleccionados", i), 0) + " de " + pdf.Num(fv(res, "N_Universo", i), 0)),
					ptxt(pdf.Num(fv(res, "Pct_AUM_Cubierto", i), 1) + "%")}
			}
			d.Tabla([]string{"Bucket", "Seleccionados / Universo", "AUM representado (informativo)"}, []string{"l", "r", "r"},
				[]float64{0.5, 0.25, 0.25}, rows)
			d.Spacer(10)
		}
		rows := make([][]pdf.Cell, cap.N)
		for i := 0; i < cap.N; i++ {
			rows[i] = []pdf.Cell{ptxt(fmt.Sprint(i + 1)), pticker(sv(cap, "Ticker", i, "")), nombreCat(cap, i), ptxt(sv(cap, "Cap_Bucket", i, "")),
				ppct(cap, "Performance 6M (%)", i, 2), ppct(cap, "Performance 1Y (%)", i, 2), pnum(cap, "Beta", i, 2),
				ppct(cap, "Alpha_CAPM_aprox", i, 2), pdf.Acento(pdf.Num(fv(cap, "Momentum_Score", i), 1)),
				ptxt(pdf.Num(fv(cap, "Peso_Portfolio", i), 2) + "%")}
		}
		d.Tabla([]string{"#", "Ticker", "Nombre / Categoría", "Bucket", "6M", "1Y", "Beta", "Alpha aprox.", "Score", "Peso"},
			[]string{"r", "l", "l", "l", "r", "r", "r", "r", "r", "r"}, []float64{0.03, 0.07, 0.27, 0.09, 0.08, 0.08, 0.07, 0.09, 0.10, 0.12}, rows)
	} else {
		d.Nota("El análisis Top 15 Cap. no se corrió en esta corrida.")
	}

	b, err := d.Bytes()
	return b, fecha, err
}

var (
	rotAligns = []string{"l", "l", "r", "r", "l"}
	rotFracs  = []float64{0.08, 0.24, 0.08, 0.09, 0.51}
)

func rotHeaders(rank string) []string {
	return []string{"Ticker", "Nombre / Categoría", rank, "Score", "Detalle"}
}

// rotRows arma las filas de nuevos ingresos o salidas (previo = usa rank/score previos y el destino).
func rotRows(filas []RotFila, previo bool) [][]pdf.Cell {
	rows := make([][]pdf.Cell, len(filas))
	for i, r := range filas {
		rank, score, detalle := pint(r.RankActual), r.Score, derefStr(r.Procedencia)
		if previo {
			rank, score, detalle = pint(r.RankPrevio), r.ScorePrevio, derefStr(r.Destino)
		}
		rows[i] = []pdf.Cell{pticker(r.Ticker), {Text: r.Nombre, Sub: r.Category, Bold: true},
			ptxt(pdf.Num(rank, 0)), ptxt(pdf.Num(score, 1)), ptxt(detalle)}
	}
	return rows
}
