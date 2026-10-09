// Package pdf es el estilo común de los informes PDF (equivale a fog_radar/core/pdf): página Letter, fondo blanco, fuente
// Carlito (sustituto métrico de Calibri, incluida en el binario), texto negro, títulos de sección en ámbar, tablas con
// encabezado claro y pie con disclaimer + «Página X de Y». Textos ≥ 10 pt, números a la derecha.
package pdf

import (
	"bytes"
	"embed"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
)

//go:embed fonts/*.ttf
var fonts embed.FS

// RGB es un color.
type RGB [3]int

// Paleta print-friendly (idéntica a core/pdf).
var (
	Blanco    = RGB{255, 255, 255}
	Negro     = RGB{0, 0, 0}
	Navy      = RGB{0x1B, 0x3A, 0x6B}
	Ambar     = RGB{0x9A, 0x6E, 0x00}
	Rojo      = RGB{0xB8, 0x32, 0x32}
	Verde     = RGB{0x1A, 0x7A, 0x55}
	Azul      = RGB{0x1B, 0x5E, 0xA5}
	Borde     = RGB{0xD8, 0xDC, 0xE3}
	CardBG    = RGB{0xF7, 0xF8, 0xFA}
	AmbarPale = RGB{0xFD, 0xFA, 0xF0}
	AzulPale  = RGB{0xF2, 0xF6, 0xFD}
)

// Disclaimer del pie de página.
const Disclaimer = "Análisis cuantitativo generado automáticamente. No constituye asesoramiento financiero."

const (
	fam        = "Carlito"
	pageW      = 792.0 // Letter apaisado
	pageH      = 612.0
	margenL    = 30.0
	margenR    = 30.0
	margenTop  = 74.0
	margenBot  = 44.0
	cellPadX   = 5.0
	cellPadY   = 4.0
	lineHeight = 12.0
)

// FrameW es el ancho útil de la página.
const FrameW = pageW - margenL - margenR

// Doc es un documento PDF apaisado con el marco estándar.
type Doc struct {
	f              *fpdf.Fpdf
	W, H           float64 // tamaño de página
	ML, MR, MT, MB float64 // márgenes
	FW             float64 // ancho útil
	PadX, PadY     float64 // relleno de las celdas de tabla
	HeaderLine     bool    // línea oscura bajo el encabezado de tabla (estilo de Tendencia)
	CenterTables   bool    // centra las tablas más angostas que el ancho útil
}

// Cell es una celda de tabla: texto principal, subtítulo opcional (9 pt navy) y estilo.
type Cell struct {
	Text  string
	Sub   string
	Bold  bool
	Color RGB // por defecto negro
	Right bool
}

// New crea el documento con título (izquierda del encabezado) y texto derecho.
func New(titulo, derecha, metaTitulo string) (*Doc, error) {
	f := fpdf.New("L", "pt", "Letter", "")
	for estilo, archivo := range map[string]string{"": "Carlito-Regular.ttf", "B": "Carlito-Bold.ttf", "I": "Carlito-Italic.ttf"} {
		b, err := fonts.ReadFile("fonts/" + archivo)
		if err != nil {
			return nil, err
		}
		f.AddUTF8FontFromBytes(fam, estilo, b)
	}
	f.SetMargins(margenL, margenTop, margenR)
	f.SetAutoPageBreak(false, margenBot)
	f.SetCellMargin(0)
	f.SetTitle(metaTitulo, true)
	f.SetAuthor("Fog Radar Quant", true)
	f.AliasNbPages("{nb}")
	f.SetHeaderFunc(func() {
		f.SetFillColor(AmbarPale[0], AmbarPale[1], AmbarPale[2])
		f.Rect(0, 0, pageW, 58, "F")
		f.SetTextColor(Navy[0], Navy[1], Navy[2])
		f.SetFont(fam, "B", 22)
		f.Text(margenL, 34, titulo)
		f.SetFont(fam, "", 11)
		f.SetTextColor(0, 0, 0)
		f.Text(pageW-margenR-f.GetStringWidth(derecha), 34, derecha)
		f.SetY(margenTop)
	})
	f.SetFooterFunc(func() {
		f.SetDrawColor(Borde[0], Borde[1], Borde[2])
		f.SetLineWidth(0.75)
		f.Line(margenL, pageH-32, pageW-margenR, pageH-32)
		f.SetFont(fam, "", 10)
		f.SetTextColor(0, 0, 0)
		f.Text(margenL, pageH-18, Disclaimer)
		pag := fmt.Sprintf("Página %d de {nb}", f.PageNo())
		f.Text(pageW-margenR-f.GetStringWidth(pag), pageH-18, pag)
	})
	f.AddPage()
	return &Doc{f: f, W: pageW, H: pageH, ML: margenL, MR: margenR, MT: margenTop, MB: margenBot, FW: FrameW, PadX: cellPadX, PadY: cellPadY}, nil
}

// PageBreak fuerza una página nueva.
func (d *Doc) PageBreak() { d.f.AddPage() }

// Spacer deja espacio vertical.
func (d *Doc) Spacer(h float64) { d.f.SetY(d.f.GetY() + h) }

func (d *Doc) ensure(h float64) bool {
	if d.f.GetY()+h > d.H-d.MB {
		d.f.AddPage()
		return true
	}
	return false
}

func (d *Doc) paragraph(texto string, estilo string, size, leading float64, c RGB, antes, despues float64) {
	d.f.SetFont(fam, estilo, size)
	lines := d.f.SplitText(texto, d.FW)
	d.ensure(antes + leading*float64(len(lines)) + despues)
	d.f.SetY(d.f.GetY() + antes)
	d.f.SetTextColor(c[0], c[1], c[2])
	for _, l := range lines {
		d.f.SetX(d.ML)
		d.f.CellFormat(d.FW, leading, l, "", 1, "L", false, 0, "")
	}
	d.f.SetY(d.f.GetY() + despues)
}

// Seccion es un título de sección (ámbar, negrita 11 pt).
func (d *Doc) Seccion(t string) { d.paragraph(t, "B", 11, 14, Ambar, 4, 6) }

// Nota es un párrafo de 11 pt negro.
func (d *Doc) Nota(t string) { d.paragraph(t, "", 11, 14, Negro, 0, 0) }

// Badge es un subtítulo en negrita de color (verde/azul/rojo).
func (d *Doc) Badge(t string, c RGB) { d.paragraph(t, "B", 11, 14, c, 6, 4) }

// Bytes cierra el documento y devuelve el PDF.
func (d *Doc) Bytes() ([]byte, error) {
	var b bytes.Buffer
	if err := d.f.Output(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (d *Doc) cellHeight(c Cell, w float64) float64 {
	st := ""
	if c.Bold {
		st = "B"
	}
	d.f.SetFont(fam, st, 10)
	h := float64(len(d.f.SplitText(c.Text, w-2*d.PadX))) * lineHeight
	if c.Sub != "" {
		d.f.SetFont(fam, "", 9)
		h += float64(len(d.f.SplitText(c.Sub, w-2*d.PadX))) * 11
	}
	return h + 2*d.PadY
}

func (d *Doc) drawCell(c Cell, x, y, w, h float64, bg RGB, header bool) {
	d.f.SetFillColor(bg[0], bg[1], bg[2])
	d.f.SetDrawColor(Borde[0], Borde[1], Borde[2])
	d.f.SetLineWidth(0.5)
	d.f.Rect(x, y, w, h, "FD")
	align := "L"
	if c.Right {
		align = "R"
	}
	st := ""
	if c.Bold || header {
		st = "B"
	}
	col := c.Color
	if header {
		col = Navy
	}
	d.f.SetFont(fam, st, 10)
	d.f.SetTextColor(col[0], col[1], col[2])
	ty := y + d.PadY
	for _, l := range d.f.SplitText(c.Text, w-2*d.PadX) {
		d.f.SetXY(x+d.PadX, ty)
		d.f.CellFormat(w-2*d.PadX, lineHeight, l, "", 0, align, false, 0, "")
		ty += lineHeight
	}
	if c.Sub != "" {
		d.f.SetFont(fam, "", 9)
		d.f.SetTextColor(Navy[0], Navy[1], Navy[2])
		for _, l := range d.f.SplitText(c.Sub, w-2*d.PadX) {
			d.f.SetXY(x+d.PadX, ty)
			d.f.CellFormat(w-2*d.PadX, 11, l, "", 0, align, false, 0, "")
			ty += 11
		}
	}
}

// Tabla dibuja una tabla con encabezado claro (repetido en cada página), grilla suave y filas alternadas.
// aligns: «l» o «r» por columna; fracs: fracción del ancho útil de cada columna.
func (d *Doc) Tabla(headers []string, aligns []string, fracs []float64, filas [][]Cell) {
	widths := make([]float64, len(fracs))
	for i, fr := range fracs {
		widths[i] = d.FW * fr
	}
	header := make([]Cell, len(headers))
	for i, h := range headers {
		header[i] = Cell{Text: h, Bold: true, Right: aligns[i] == "r"}
	}
	total := 0.0
	for _, w := range widths {
		total += w
	}
	x0 := d.ML
	if d.CenterTables && total < d.FW {
		x0 += (d.FW - total) / 2
	}
	drawRow := func(row []Cell, bg RGB, isHeader bool) {
		h := 0.0
		for i, c := range row {
			h = math.Max(h, d.cellHeight(c, widths[i]))
		}
		y := d.f.GetY()
		x := x0
		for i, c := range row {
			d.drawCell(c, x, y, widths[i], h, bg, isHeader)
			x += widths[i]
		}
		if isHeader && d.HeaderLine {
			d.f.SetDrawColor(Navy[0], Navy[1], Navy[2])
			d.f.SetLineWidth(1)
			d.f.Line(x0, y+h, x0+total, y+h)
		}
		d.f.SetXY(d.ML, y+h)
	}
	rowHeight := func(row []Cell) float64 {
		h := 0.0
		for i, c := range row {
			h = math.Max(h, d.cellHeight(c, widths[i]))
		}
		return h
	}
	// El encabezado y la primera fila no se separan.
	need := rowHeight(header)
	if len(filas) > 0 {
		need += rowHeight(filas[0])
	}
	d.ensure(need)
	drawRow(header, CardBG, true)
	for i, row := range filas {
		for j := range row {
			row[j].Right = aligns[j] == "r"
		}
		if d.f.GetY()+rowHeight(row) > d.H-d.MB {
			d.f.AddPage()
			drawRow(header, CardBG, true)
		}
		bg := Blanco
		if (i+1)%2 == 0 {
			bg = CardBG
		}
		drawRow(row, bg, false)
	}
}

// --- Formato ---------------------------------------------------------------------------------------------------------

// Vacio es el guion largo de los valores faltantes.
const Vacio = "—"

// Num formatea con separador de miles y dec decimales; v nil → «—».
func Num(v *float64, dec int) string {
	if v == nil || math.IsNaN(*v) {
		return Vacio
	}
	return group(strconv.FormatFloat(*v, 'f', dec, 64))
}

func group(s string) string {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	ent, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, r := range ent {
		if i > 0 && (len(ent)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	out := b.String()
	if frac != "" {
		out += "." + frac
	}
	if neg {
		out = "-" + out
	}
	return out
}

// Pct formatea un porcentaje ya expresado en puntos (12.3 → «+12.30%»).
func Pct(v *float64, dec int) string {
	if v == nil || math.IsNaN(*v) {
		return Vacio
	}
	signo := ""
	if *v > 0 {
		signo = "+"
	}
	return signo + Num(v, dec) + "%"
}

// FechaYYYYMMDD: «20260924» → «24/09/2026» (otros formatos se devuelven tal cual).
func FechaYYYYMMDD(s string) string {
	s, _, _ = strings.Cut(s, ".")
	if len(s) == 8 {
		if _, err := strconv.Atoi(s); err == nil {
			return s[6:8] + "/" + s[4:6] + "/" + s[0:4]
		}
	}
	if s == "" {
		return Vacio
	}
	return s
}

// Signo es una celda numérica verde/roja según el signo (solo para datos clave: deltas, z-scores).
func Signo(v *float64, dec int) Cell {
	if v == nil || math.IsNaN(*v) {
		return Cell{Text: Vacio}
	}
	c := Negro
	switch {
	case *v > 0:
		c = Verde
	case *v < 0:
		c = Rojo
	}
	signo := ""
	if *v > 0 {
		signo = "+"
	}
	return Cell{Text: signo + Num(v, dec), Bold: true, Color: c}
}

// Acento es una celda destacada en ámbar (score principal de cada tabla).
func Acento(texto string) Cell { return Cell{Text: texto, Bold: true, Color: Ambar} }
