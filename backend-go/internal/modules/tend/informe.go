package tend

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fogradar/internal/pdf"
)

// Informe PDF de la corrida: resumen ejecutivo, portafolio seleccionado + gráfico, proyección a
// 6 meses + gráfico, sector en tendencia y top momentum mensual, una sección por análisis, más contexto de mercado y
// limitaciones. Vertical (Letter). Los gráficos se dibujan directamente.

var mesesES = []string{"", "enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

const (
	inch      = 72.0
	boxWidth  = 6.8 * inch
	cardWidth = 1.6 * inch
)

func fnum(o OM, k string) (float64, bool) {
	switch v := o.Get(k).(type) {
	case float64:
		return v, !math.IsNaN(v)
	case int64:
		return float64(v), true
	}
	return 0, false
}

func fstr(o OM, k string) string {
	if s, ok := o.Get(k).(string); ok {
		return s
	}
	return ""
}

func fmtPct1(o OM, k string) string {
	if v, ok := fnum(o, k); ok {
		return fmt.Sprintf("%.1f%%", v)
	}
	return pdf.Vacio
}

func fmtUSD(o OM, k string) string {
	if v, ok := fnum(o, k); ok {
		return fmt.Sprintf("$%.2f", v)
	}
	return pdf.Vacio
}

func signalColor(s string) pdf.RGB {
	switch s {
	case "Alcista":
		return pdf.Verde
	case "Bajista":
		return pdf.Rojo
	case "Lateral / mixta":
		return pdf.Ambar
	case "Datos insuficientes":
		return pdf.Borde
	}
	return pdf.Negro
}

func rows(v any) []OM {
	var out []OM
	for _, x := range asList(v) {
		if o, ok := x.(OM); ok {
			out = append(out, o)
		}
	}
	return out
}

func chartHeight(n int) float64 { return math.Max(3.6, 0.27*float64(n)+1.0) * inch }

func narrativeSegs(sectors []OM, n int) []pdf.Seg {
	top := sectors[0]
	byWeight := sectors[0]
	for _, s := range sectors {
		a, _ := fnum(s, "peso_total")
		b, _ := fnum(byWeight, "peso_total")
		if a > b {
			byWeight = s
		}
	}
	ns, _ := fnum(top, "n_acciones")
	pt, _ := fnum(top, "peso_total")
	rp, _ := fnum(top, "retorno_6m_prom")
	sec, sig, tk := fstr(top, "sector"), fstr(top, "senal_predominante"), fstr(top, "tickers")
	if sec == fstr(byWeight, "sector") {
		return []pdf.Seg{pdf.B(sec), pdf.T(fmt.Sprintf(" es el sector con mejor proyección dentro de la selección: agrupa %d de las %d acciones (%.1f%% del portafolio), con un retorno proyectado a 6 meses promedio de %.1f%% y señal técnica predominantemente ", int(ns), n, pt*100, rp)),
			pdf.B(sig), pdf.T(fmt.Sprintf(" (%s). Además es el sector con mayor peso asignado por el modelo, así que la concentración de capital y la mejor proyección coinciden.", tk))}
	}
	wp, _ := fnum(byWeight, "peso_total")
	return []pdf.Seg{pdf.B(sec), pdf.T(fmt.Sprintf(" es el sector con mejor proyección a 6 meses dentro de la selección (retorno promedio %.1f%%, señal predominante ", rp)),
		pdf.B(sig), pdf.T(fmt.Sprintf("; %s). En peso asignado por el modelo, sin embargo, el sector dominante es ", tk)), pdf.B(fstr(byWeight, "sector")),
		pdf.T(fmt.Sprintf(" (%.1f%% del portafolio) — es decir, el modelo concentró más capital ahí por su menor volatilidad/correlación, aunque no sea el de mayor retorno proyectado.", wp*100))}
}

func cell(t string) pdf.Cell { return pdf.Cell{Text: t} }

func universeSection(d *pdf.Doc, perf OM, holdings, sectors, momentum []OM, intro, momIntro, suffix string, nSel int) {
	pf, _ := perf.Get("portafolio_15_finales").(OM)
	ret, _ := fnum(pf, "retorno_esperado_anual")
	vol, _ := fnum(pf, "volatilidad_anual")
	sh, _ := fnum(pf, "sharpe_ratio")
	rf, _ := fnum(perf, "tasa_libre_riesgo_anual")

	d.Titulo("RESUMEN EJECUTIVO" + suffix)
	d.Tarjetas([]pdf.Tarjeta{
		{Label: "Retorno esperado anual", Value: fmt.Sprintf("%.1f%%", ret*100), Color: pdf.Verde},
		{Label: "Volatilidad anual", Value: fmt.Sprintf("%.1f%%", vol*100), Color: pdf.Azul},
		{Label: "Sharpe Ratio", Value: fmt.Sprintf("%.2f", sh), Color: pdf.Ambar},
		{Label: "Tasa libre de riesgo", Value: fmt.Sprintf("%.2f%%", rf*100), Color: pdf.Negro},
	}, cardWidth)
	d.Spacer(10)
	d.Parrafo([]pdf.Seg{pdf.T(intro)}, 11, 15, pdf.Negro, 0, 6)

	d.Titulo(fmt.Sprintf("PORTAFOLIO SELECCIONADO%s — %d ACCIONES", suffix, len(holdings)))
	pw := []float64{0.78, 1.35, 0.55, 1.4, 0.6, 0.9, 0.9}
	prow := make([][]pdf.Cell, len(holdings))
	for i, h := range holdings {
		peso, _ := fnum(h, "peso_final")
		sig := fstr(h, "senal_tecnica")
		prow[i] = []pdf.Cell{cell(fstr(h, "ticker")), cell(fstr(h, "nombre")), cell(fstr(h, "pais")), cell(fstr(h, "sector")),
			cell(fmt.Sprintf("%.1f%%", peso*100)), cell(fmtPct1(h, "retorno_esperado_anual_pct")), {Text: sig, Bold: true, Color: signalColor(sig)}}
	}
	d.Tabla([]string{"Ticker", "Empresa", "País", "Sector", "Peso", "Retorno anual", "Señal 6m"}, []string{"l", "l", "l", "l", "r", "r", "l"}, fracs(pw), prow)
	d.Spacer(10)

	asc := append([]OM(nil), holdings...)
	sort.SliceStable(asc, func(a, b int) bool {
		x, _ := fnum(asc[a], "peso_final")
		y, _ := fnum(asc[b], "peso_final")
		return x < y
	})
	labels, vals, cols := make([]string, len(asc)), make([]float64, len(asc)), make([]pdf.RGB, len(asc))
	for i, h := range asc {
		p, _ := fnum(h, "peso_final")
		labels[i], vals[i], cols[i] = fstr(h, "nombre"), p*100, pdf.Azul
	}
	d.Barras(labels, vals, cols, "Peso en el portafolio (%)", nil, boxWidth, chartHeight(len(asc)))

	d.Titulo("PROYECCIÓN DE TENDENCIA A 6 MESES" + suffix)
	d.Parrafo([]pdf.Seg{pdf.T("Proyección estadística tipo movimiento geométrico browniano (GBM), a partir del retorno y volatilidad histórica anualizada de cada acción. El precio central es el escenario más probable; el rango min-máx corresponde a una banda de confianza aproximada del 90%. "),
		pdf.B("No es una predicción garantizada"), pdf.T(": es un rango de escenarios estadísticos basado en el comportamiento de los últimos 5 años.")}, 11, 15, pdf.Negro, 0, 6)
	fw := []float64{0.8, 1.15, 1.15, 2.2, 1.1}
	frow := make([][]pdf.Cell, len(holdings))
	for i, h := range holdings {
		frow[i] = []pdf.Cell{cell(fstr(h, "ticker")), cell(fmtUSD(h, "precio_actual_usd")), cell(fmtUSD(h, "precio_proyectado_6m_usd")),
			cell(fmtUSD(h, "precio_min_90pct_6m_usd") + " – " + fmtUSD(h, "precio_max_90pct_6m_usd")), cell(fmtPct1(h, "retorno_esperado_6m_pct"))}
	}
	d.Tabla([]string{"Ticker", "Precio actual", "Proyectado 6m", "Rango 90%", "Retorno 6m"}, []string{"l", "r", "r", "r", "r"}, fracs(fw), frow)
	d.Spacer(10)

	asc = append([]OM(nil), holdings...)
	sort.SliceStable(asc, func(a, b int) bool {
		x, _ := fnum(asc[a], "retorno_esperado_6m_pct")
		y, _ := fnum(asc[b], "retorno_esperado_6m_pct")
		return x < y
	})
	for i, h := range asc {
		r, _ := fnum(h, "retorno_esperado_6m_pct")
		labels[i], vals[i], cols[i] = fstr(h, "nombre"), r, signalColor(fstr(h, "senal_tecnica"))
	}
	d.Barras(labels, vals, cols, "Retorno proyectado a 6 meses (%, escenario central)",
		[]pdf.LeyendaBarra{{Label: "Alcista", Color: pdf.Verde}, {Label: "Lateral / mixta", Color: pdf.Ambar}, {Label: "Bajista", Color: pdf.Rojo}},
		boxWidth, chartHeight(len(asc)))

	d.Titulo("SECTOR EN TENDENCIA" + suffix)
	if len(sectors) > 0 {
		d.Parrafo(narrativeSegs(sectors, nSel), 11, 15, pdf.Negro, 0, 6)
		d.Spacer(6)
		sw := []float64{2.3, 0.85, 0.95, 1.2, 1.5}
		srow := make([][]pdf.Cell, len(sectors))
		for i, s := range sectors {
			n, _ := fnum(s, "n_acciones")
			pt, _ := fnum(s, "peso_total")
			sig := fstr(s, "senal_predominante")
			srow[i] = []pdf.Cell{cell(fstr(s, "sector")), cell(fmt.Sprint(int(n))), cell(fmt.Sprintf("%.1f%%", pt*100)),
				cell(fmtPct1(s, "retorno_6m_prom")), {Text: sig, Bold: true, Color: signalColor(sig)}}
		}
		d.Tabla([]string{"Sector", "N° acciones", "Peso total", "Retorno 6m prom.", "Señal predominante"}, []string{"l", "r", "r", "r", "l"}, fracs(sw), srow)
	}

	if len(momentum) > 0 {
		d.Titulo("TOP ACCIONES POR MOMENTUM MENSUAL" + suffix)
		d.Parrafo([]pdf.Seg{pdf.T(momIntro)}, 11, 15, pdf.Negro, 0, 6)
		mw := []float64{0.35, 0.9, 1.75, 0.72, 0.72, 0.72, 0.72, 0.72}
		mrow := make([][]pdf.Cell, len(momentum))
		for i, m := range momentum {
			rk, _ := fnum(m, "rank")
			score, ok := fnum(m, "momentum_score_pct")
			sc := pdf.Cell{Text: pdf.Vacio, Bold: true, Color: pdf.Rojo}
			if ok {
				sc.Text = fmt.Sprintf("%.1f%%", score)
				if score >= 0 {
					sc.Color = pdf.Verde
				}
			}
			mrow[i] = []pdf.Cell{cell(fmt.Sprint(int(rk))), cell(fstr(m, "ticker")), cell(fstr(m, "nombre")),
				cell(fmtPct1(m, "momentum_1m_pct")), cell(fmtPct1(m, "momentum_3m_pct")), cell(fmtPct1(m, "momentum_6m_pct")),
				cell(fmtPct1(m, "momentum_12m_pct")), sc}
		}
		d.Tabla([]string{"#", "Ticker", "Empresa", "1m", "3m", "6m", "12m", "Score"}, []string{"r", "l", "l", "r", "r", "r", "r", "r"}, fracs(mw), mrow)
		d.Spacer(10)
	}
}

// fracs convierte anchos en pulgadas a fracciones del ancho útil (7.3 in).
func fracs(inches []float64) []float64 {
	out := make([]float64, len(inches))
	for i, w := range inches {
		out[i] = w * inch / (612 - 2*0.6*inch)
	}
	return out
}

// GenerarPDF arma informe_portafolio_<YYYYMMDD>.pdf en la carpeta de la corrida y devuelve su ruta.
func GenerarPDF(runDir string, u *Universe, p Params, hasMacro bool, now time.Time) (string, error) {
	hoy := fmt.Sprintf("%d de %s de %d", now.Day(), mesesES[now.Month()], now.Year())
	d, err := pdf.NewPortrait("Informe de Portafolio - Modelo Markowitz Asia + EEUU", 0.6*inch, 0.6*inch, 1.15*inch, 0.8*inch,
		func(d *pdf.Doc) {
			f := d.F()
			f.SetFillColor(pdf.AmbarPale[0], pdf.AmbarPale[1], pdf.AmbarPale[2])
			f.Rect(0, 0, 612, 0.95*inch, "F")
			f.SetDrawColor(pdf.Borde[0], pdf.Borde[1], pdf.Borde[2])
			f.SetLineWidth(0.75)
			f.Line(0.6*inch, 0.95*inch, 612-0.6*inch, 0.95*inch)
			d.Font("B", 13)
			f.SetTextColor(0, 0, 0)
			f.Text(0.6*inch, 0.55*inch, "Informe de Portafolio — Modelo Markowitz Asia + EEUU")
			d.Font("", 11)
			f.SetTextColor(pdf.Navy[0], pdf.Navy[1], pdf.Navy[2])
			f.Text(0.6*inch, 0.75*inch, hoy)
		},
		func(d *pdf.Doc) {
			f := d.F()
			f.SetTextColor(pdf.Navy[0], pdf.Navy[1], pdf.Navy[2])
			d.Font("I", 11)
			f.Text(0.6*inch, 792-0.5*inch, "Informe generado automáticamente con fines informativos. No constituye asesoría financiera.")
			d.Font("", 11)
			pag := fmt.Sprintf("Página %d", f.PageNo())
			f.Text(612-0.6*inch-f.GetStringWidth(pag), 792-0.5*inch, pag)
		})
	if err != nil {
		return "", err
	}

	first := true
	primarySharpe := math.NaN()
	for _, spec := range Analyses(u, p) {
		a, err := AnalysisPayload(runDir, spec, u, p)
		if err != nil {
			return "", err
		}
		if a == nil {
			continue
		}
		perf, _ := a.Get("performance").(OM)
		nSel := len(asList(perf.Get("tickers_seleccionados")))
		if first {
			if notes := readNotes(runDir); len(notes) > 0 {
				items := make([][]pdf.Seg, len(notes))
				for i, n := range notes {
					items[i] = []pdf.Seg{pdf.T(n.(string))}
				}
				d.Titulo("CONTEXTO DE MERCADO")
				d.Caja(pdf.AzulPale, items, 11, 14, 12, 10, boxWidth)
				d.Spacer(10)
			}
		} else {
			d.PageBreak()
			d.Parrafo([]pdf.Seg{pdf.B("ANÁLISIS COMPLEMENTARIO — " + strings.ToUpper(spec.Label))}, 22, 26, pdf.Negro, 0, 2)
			d.Parrafo([]pdf.Seg{pdf.T(fmt.Sprintf("Mismo modelo y metodología del análisis anterior, aplicado únicamente al universo '%s', para comparar cómo se ve el portafolio bajo ese universo más acotado.", spec.Label))}, 11, 14, pdf.Navy, 0, 0)
			d.Spacer(10)
		}
		suffix := ""
		if !first {
			suffix = " (" + strings.ToUpper(spec.Label) + ")"
		}
		intro := fmt.Sprintf("El modelo de Markowitz/covarianza, aplicado sobre el universo '%s' (%d acciones, universo %s) con 5 años de historial semanal de precios en USD, seleccionó las %d acciones que maximizan el Sharpe Ratio del portafolio.", spec.Label, len(spec.Tickers), u.Source, nSel)
		momIntro := fmt.Sprintf("Ranking independiente de la selección de Markowitz: evalúa el universo '%s' (%d acciones) por fuerza y persistencia de la tendencia mensual (retorno acumulado a 1, 3, 6 y 12 meses, ponderado por la proporción de meses positivos del último año). Puede incluir acciones que el modelo de covarianza descartó por volatilidad o correlación alta, pero que muestran el mejor impulso de corto plazo.", spec.Label, len(spec.Tickers))
		universeSection(d, perf, rows(a.Get("holdings")), rows(a.Get("sectors")), rows(a.Get("momentum")), intro, momIntro, suffix, nSel)
		if spec.Key == "" {
			pf, _ := perf.Get("portafolio_15_finales").(OM)
			primarySharpe, _ = fnum(pf, "sharpe_ratio")
		}
		first = false
	}
	if math.IsNaN(primarySharpe) {
		return "", fmt.Errorf("no se generó ningún análisis (ver Markowitz); no hay nada que reportar")
	}

	rfNote := "La tasa libre de riesgo usada proviene de ^IRX (Yahoo) como respaldo, no de FRED, porque los datos macro no se habían descargado con una API key de FRED al momento de este informe."
	if hasMacro {
		rfNote = "La tasa libre de riesgo y el contexto de mercado (curva de tasas, spreads de crédito) provienen de datos reales de FRED."
	}
	limits := []string{
		"Este informe es un producto cuantitativo (MVP) y no constituye asesoría financiera ni recomendación de inversión. Consulta a un asesor financiero antes de tomar decisiones.",
		"El modelo de Markowitz usa retornos y covarianzas históricos de 5 años como estimador del comportamiento futuro; esto no garantiza que el mismo patrón se repita, y tiende a sobreponderar activos que tuvieron un desempeño excepcional recientemente.",
		fmt.Sprintf("El Sharpe Ratio del portafolio principal (%.2f) refleja el período de mercado usado para estimarlo; no debe interpretarse como un retorno esperado garantizado hacia adelante. Lo mismo aplica a cualquier análisis complementario incluido en este informe.", primarySharpe),
		rfNote,
	}
	items := make([][]pdf.Seg, len(limits))
	for i, l := range limits {
		items[i] = []pdf.Seg{pdf.T("• " + l)}
	}
	d.Titulo("LIMITACIONES Y ADVERTENCIAS")
	d.Caja(pdf.AmbarPale, items, 11, 14, 12, 10, boxWidth)

	b, err := d.Bytes()
	if err != nil {
		return "", err
	}
	out := filepath.Join(runDir, "informe_portafolio_"+now.Format("20060102")+".pdf")
	return out, os.WriteFile(out, b, 0o644)
}
