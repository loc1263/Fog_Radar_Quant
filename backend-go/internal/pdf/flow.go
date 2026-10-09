package pdf

import (
	"fmt"
	"math"
	"strings"

	"github.com/go-pdf/fpdf"
)

// Elementos adicionales del flujo (usados por el informe de Tendencia, vertical): texto con negritas, cajas con fondo,
// tarjetas de métricas y gráfico de barras horizontales dibujado directamente (sin imágenes).

// Seg es un trozo de texto con estilo.
type Seg struct {
	Text  string
	Bold  bool
	Color *RGB
}

// B es un trozo en negrita.
func B(t string) Seg { return Seg{Text: t, Bold: true} }

// T es un trozo normal.
func T(t string) Seg { return Seg{Text: t} }

// NewPortrait crea un documento vertical Letter. header y footer dibujan el marco de cada página (el cursor queda en el
// margen superior después del encabezado).
func NewPortrait(meta string, ml, mr, mt, mb float64, header, footer func(d *Doc)) (*Doc, error) {
	f := fpdf.New("P", "pt", "Letter", "")
	for estilo, archivo := range map[string]string{"": "Carlito-Regular.ttf", "B": "Carlito-Bold.ttf", "I": "Carlito-Italic.ttf"} {
		b, err := fonts.ReadFile("fonts/" + archivo)
		if err != nil {
			return nil, err
		}
		f.AddUTF8FontFromBytes(fam, estilo, b)
	}
	f.SetMargins(ml, mt, mr)
	f.SetAutoPageBreak(false, mb)
	f.SetCellMargin(0)
	f.SetTitle(meta, true)
	f.SetAuthor("Fog Radar Quant", true)
	f.AliasNbPages("{nb}")
	d := &Doc{f: f, W: 612, H: 792, ML: ml, MR: mr, MT: mt, MB: mb, FW: 612 - ml - mr, PadX: 6, PadY: 6, HeaderLine: true, CenterTables: true}
	f.SetHeaderFunc(func() {
		header(d)
		f.SetY(mt)
	})
	f.SetFooterFunc(func() { footer(d) })
	f.AddPage()
	return d, nil
}

// F expone el documento fpdf (para encabezados y pies propios).
func (d *Doc) F() *fpdf.Fpdf { return d.f }

// Font fija fuente, estilo («», «B», «I») y tamaño.
func (d *Doc) Font(style string, size float64) { d.f.SetFont(fam, style, size) }

type word struct {
	text string
	seg  Seg
	glue bool // sin espacio antes (el texto sigue pegado al trozo anterior)
}

// wrap parte segmentos de texto en líneas de ancho ≤ w (corte por palabras).
func (d *Doc) wrap(segs []Seg, size, w float64) [][]word {
	var lines [][]word
	var cur []word
	curW := 0.0
	style := func(s Seg) string {
		if s.Bold {
			return "B"
		}
		return ""
	}
	space := func() float64 { d.f.SetFont(fam, "", size); return d.f.GetStringWidth(" ") }
	prevEndsSpace := true // al inicio no hay nada que pegar
	for _, s := range segs {
		startsSpace := strings.HasPrefix(s.Text, " ") || strings.HasPrefix(s.Text, "\n")
		first := true
		for _, part := range strings.Split(s.Text, "\n") {
			for _, wd := range strings.Fields(part) {
				d.f.SetFont(fam, style(s), size)
				ww := d.f.GetStringWidth(wd)
				glue := first && !prevEndsSpace && !startsSpace
				first = false
				add := ww
				if len(cur) > 0 && !glue {
					add += space()
				}
				if len(cur) > 0 && curW+add > w {
					lines = append(lines, cur)
					cur, curW, add, glue = nil, 0, ww, false
				}
				cur = append(cur, word{wd, s, glue})
				curW += add
			}
		}
		prevEndsSpace = strings.HasSuffix(s.Text, " ") || strings.HasSuffix(s.Text, "\n")
	}
	if len(cur) > 0 {
		lines = append(lines, cur)
	}
	return lines
}

// drawLines dibuja líneas ya partidas a partir de (x, y) (y = arriba de la primera línea).
func (d *Doc) drawLines(lines [][]word, x, y, size, leading float64, base RGB) {
	for _, ln := range lines {
		cx := x
		for i, wd := range ln {
			st := ""
			if wd.seg.Bold {
				st = "B"
			}
			d.f.SetFont(fam, st, size)
			c := base
			if wd.seg.Color != nil {
				c = *wd.seg.Color
			}
			d.f.SetTextColor(c[0], c[1], c[2])
			if i > 0 && !wd.glue {
				d.f.SetFont(fam, "", size)
				cx += d.f.GetStringWidth(" ")
				d.f.SetFont(fam, st, size)
			}
			d.f.Text(cx, y+size*0.82+(leading-size)/2, wd.text)
			cx += d.f.GetStringWidth(wd.text)
		}
		y += leading
	}
}

// Parrafo escribe texto con negritas (tamaño y interlineado dados) con espacios antes/después.
func (d *Doc) Parrafo(segs []Seg, size, leading float64, c RGB, antes, despues float64) {
	lines := d.wrap(segs, size, d.FW)
	d.ensure(antes + leading*float64(len(lines)) + despues)
	y := d.f.GetY() + antes
	d.drawLines(lines, d.ML, y, size, leading, c)
	d.f.SetY(y + leading*float64(len(lines)) + despues)
}

// Titulo es un título de sección con espacio antes de 14 pt (estilo SectionHeader de Tendencia).
func (d *Doc) Titulo(t string) {
	d.ensure(14 + 14 + 6 + 40) // el título no queda huérfano al pie de página
	d.paragraph(t, "B", 11, 14, Ambar, 14, 6)
}

// Caja dibuja párrafos dentro de un recuadro con fondo y borde (notas, advertencias).
func (d *Doc) Caja(bg RGB, items [][]Seg, size, leading, padX, padY, width float64) {
	var all [][][]word
	h := 2 * padY
	for _, it := range items {
		ln := d.wrap(it, size, width-2*padX)
		all = append(all, ln)
		h += float64(len(ln)) * leading
	}
	d.ensure(h)
	y := d.f.GetY()
	x := d.ML + (d.FW-width)/2
	d.f.SetFillColor(bg[0], bg[1], bg[2])
	d.f.SetDrawColor(Borde[0], Borde[1], Borde[2])
	d.f.SetLineWidth(0.75)
	d.f.Rect(x, y, width, h, "FD")
	ty := y + padY
	for _, ln := range all {
		d.drawLines(ln, x+padX, ty, size, leading, Negro)
		ty += float64(len(ln)) * leading
	}
	d.f.SetY(y + h)
}

// Tarjeta es una métrica con etiqueta y valor grande.
type Tarjeta struct {
	Label, Value string
	Color        RGB
}

// Tarjetas dibuja una fila de tarjetas de ancho colW (con 3 pt de separación a cada lado).
func (d *Doc) Tarjetas(cards []Tarjeta, colW float64) {
	const padX, padY = 12.0, 12.0
	inner := colW - 6 - 2*padX
	type laid struct {
		lines [][]word
		h     float64
	}
	var ls []laid
	maxH := 0.0
	for _, c := range cards {
		lines := d.wrap([]Seg{T(c.Label)}, 11, inner)
		h := 2*padY + float64(len(lines))*13 + 22
		ls = append(ls, laid{lines, h})
		maxH = math.Max(maxH, h)
	}
	d.ensure(maxH)
	y := d.f.GetY()
	total := colW * float64(len(cards))
	x0 := d.ML + (d.FW-total)/2
	for i, c := range cards {
		x := x0 + float64(i)*colW + 3
		w := colW - 6
		d.f.SetFillColor(CardBG[0], CardBG[1], CardBG[2])
		d.f.SetDrawColor(Borde[0], Borde[1], Borde[2])
		d.f.SetLineWidth(0.75)
		d.f.Rect(x, y, w, ls[i].h, "FD")
		d.drawLines(ls[i].lines, x+padX, y+padY, 11, 13, Navy)
		d.f.SetFont(fam, "B", 18)
		d.f.SetTextColor(c.Color[0], c.Color[1], c.Color[2])
		d.f.Text(x+padX, y+padY+float64(len(ls[i].lines))*13+22*0.8, c.Value)
	}
	d.f.SetY(y + maxH)
}

// LeyendaBarra es un elemento de la leyenda de un gráfico.
type LeyendaBarra struct {
	Label string
	Color RGB
}

// Barras dibuja un gráfico de barras horizontales (etiquetas a la izquierda, valor al final de cada barra, grilla vertical
// suave y leyenda opcional abajo a la derecha). Las barras van de abajo hacia arriba en el orden dado (como matplotlib).
func (d *Doc) Barras(labels []string, values []float64, colors []RGB, xlabel string, leyenda []LeyendaBarra, width, height float64) {
	d.ensure(height)
	x0 := d.ML + (d.FW-width)/2
	y0 := d.f.GetY()
	// Área de etiquetas: la más ancha (10 pt), con tope del 38 % del ancho.
	d.f.SetFont(fam, "", 10)
	labW := 0.0
	for _, l := range labels {
		labW = math.Max(labW, d.f.GetStringWidth(l))
	}
	labW = math.Min(labW, width*0.38)
	left := x0 + labW + 10
	right := x0 + width - 38 // margen para el valor de la barra más larga
	top := y0 + 6
	bottom := y0 + height - 34 // espacio para eje y xlabel

	lo, hi := 0.0, 0.0
	for _, v := range values {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	if hi == lo {
		hi = lo + 1
	}
	step := niceStep((hi - lo) / 5)
	lo = math.Floor(lo/step) * step
	hi = math.Ceil(hi/step) * step
	sx := func(v float64) float64 { return left + (v-lo)/(hi-lo)*(right-left) }

	// grilla y ticks
	d.f.SetLineWidth(0.4)
	for v := lo; v <= hi+step/2; v += step {
		d.f.SetDrawColor(Borde[0], Borde[1], Borde[2])
		d.f.Line(sx(v), top, sx(v), bottom)
		d.f.SetFont(fam, "", 10)
		d.f.SetTextColor(0, 0, 0)
		t := fmt.Sprintf("%g", math.Round(v*100)/100)
		d.f.Text(sx(v)-d.f.GetStringWidth(t)/2, bottom+12, t)
	}
	d.f.SetLineWidth(0.75)
	d.f.Line(left, top, left, bottom)
	d.f.Line(left, bottom, right, bottom)

	n := len(values)
	rowH := (bottom - top) / float64(n)
	for i, v := range values {
		cy := bottom - (float64(i)+0.5)*rowH
		barH := rowH * 0.6
		c := colors[i]
		d.f.SetFillColor(c[0], c[1], c[2])
		a, b := sx(0), sx(v)
		d.f.Rect(math.Min(a, b), cy-barH/2, math.Abs(b-a), barH, "F")
		d.f.SetFont(fam, "", 10)
		d.f.SetTextColor(0, 0, 0)
		lbl := labels[i]
		for d.f.GetStringWidth(lbl) > labW && len(lbl) > 1 {
			lbl = lbl[:len(lbl)-1]
		}
		d.f.Text(left-6-d.f.GetStringWidth(lbl), cy+3.5, lbl)
		d.f.SetTextColor(Navy[0], Navy[1], Navy[2])
		txt := fmt.Sprintf("%.1f%%", v)
		if v >= 0 {
			d.f.Text(b+4, cy+3.5, txt)
		} else {
			d.f.Text(b-4-d.f.GetStringWidth(txt), cy+3.5, txt)
		}
	}
	d.f.SetFont(fam, "", 10)
	d.f.SetTextColor(0, 0, 0)
	d.f.Text((left+right)/2-d.f.GetStringWidth(xlabel)/2, y0+height-6, xlabel)

	// leyenda abajo a la derecha, dentro del área
	ly := bottom - 8
	for i := len(leyenda) - 1; i >= 0; i-- {
		l := leyenda[i]
		d.f.SetFont(fam, "", 10)
		tw := d.f.GetStringWidth(l.Label)
		d.f.SetFillColor(255, 255, 255)
		d.f.Rect(right-tw-26, ly-9, tw+24, 13, "F")
		d.f.SetFillColor(l.Color[0], l.Color[1], l.Color[2])
		d.f.Rect(right-tw-22, ly-7, 12, 8, "F")
		d.f.SetTextColor(0, 0, 0)
		d.f.Text(right-tw-6, ly, l.Label)
		ly -= 14
	}
	d.f.SetY(y0 + height)
}

// niceStep redondea un paso de eje a 1, 2 o 5 × 10^k.
func niceStep(raw float64) float64 {
	if raw <= 0 {
		return 1
	}
	exp := math.Floor(math.Log10(raw))
	f := raw / math.Pow(10, exp)
	var n float64
	switch {
	case f <= 1:
		n = 1
	case f <= 2:
		n = 2
	case f <= 5:
		n = 5
	default:
		n = 10
	}
	return n * math.Pow(10, exp)
}
