package quant

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Kind es el tipo de una columna: determina cómo se escribe al CSV.
type Kind uint8

const (
	// Float: float64 con NaN como faltante («1.0», «0.5», vacío).
	Float Kind = iota
	// Int: int64 sin faltantes («13»).
	Int
	// Str: texto con faltantes explícitos.
	Str
)

// Column es una columna tipada de un Frame.
type Column struct {
	Name string
	Kind Kind
	F    []float64 // Kind == Float
	I    []int64   // Kind == Int
	S    []string  // Kind == Str
	Null []bool    // Kind == Str: true = faltante
}

// Len es el número de filas.
func (c *Column) Len() int {
	switch c.Kind {
	case Int:
		return len(c.I)
	case Str:
		return len(c.S)
	}
	return len(c.F)
}

// IsNull dice si la fila i es faltante.
func (c *Column) IsNull(i int) bool {
	switch c.Kind {
	case Int:
		return false
	case Str:
		return c.Null[i]
	}
	return math.IsNaN(c.F[i])
}

// Floats devuelve la columna como []float64 (texto no numérico → NaN), como pd.to_numeric(errors="coerce").
func (c *Column) Floats() []float64 {
	out := make([]float64, c.Len())
	for i := range out {
		switch c.Kind {
		case Float:
			out[i] = c.F[i]
		case Int:
			out[i] = float64(c.I[i])
		default:
			out[i] = NaN
			if !c.Null[i] {
				if f, err := strconv.ParseFloat(strings.TrimSpace(c.S[i]), 64); err == nil {
					out[i] = f
				}
			}
		}
	}
	return out
}

// At devuelve la fila i como float64 (NaN si es faltante o texto no numérico).
func (c *Column) At(i int) float64 {
	switch c.Kind {
	case Float:
		return c.F[i]
	case Int:
		return float64(c.I[i])
	}
	if c.Null[i] {
		return NaN
	}
	if f, err := strconv.ParseFloat(strings.TrimSpace(c.S[i]), 64); err == nil {
		return f
	}
	return NaN
}

// Cell devuelve el texto CSV de la fila i, en el formato estable de CSV.
func (c *Column) Cell(i int) string {
	switch c.Kind {
	case Int:
		return strconv.FormatInt(c.I[i], 10)
	case Str:
		if c.Null[i] {
			return ""
		}
		return c.S[i]
	}
	if math.IsNaN(c.F[i]) {
		return ""
	}
	return FloatRepr(c.F[i])
}

// Value es el valor de la fila i para JSON: nil, float64, int64 o string.
func (c *Column) Value(i int) any {
	switch c.Kind {
	case Int:
		return c.I[i]
	case Str:
		if c.Null[i] {
			return nil
		}
		return c.S[i]
	}
	if math.IsNaN(c.F[i]) {
		return nil
	}
	return c.F[i]
}

func (c *Column) take(idx []int) *Column {
	out := &Column{Name: c.Name, Kind: c.Kind}
	for _, i := range idx {
		switch c.Kind {
		case Int:
			out.I = append(out.I, c.I[i])
		case Str:
			out.S = append(out.S, c.S[i])
			out.Null = append(out.Null, c.Null[i])
		default:
			out.F = append(out.F, c.F[i])
		}
	}
	return out
}

// Frame es una tabla de columnas tipadas con orden de columnas (un DataFrame mínimo).
type Frame struct {
	Cols []*Column
	N    int
}

// NewFrame crea una tabla vacía de n filas.
func NewFrame(n int) *Frame { return &Frame{N: n} }

// Col busca una columna por nombre.
func (f *Frame) Col(name string) *Column {
	for _, c := range f.Cols {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// Has dice si existe la columna.
func (f *Frame) Has(name string) bool { return f.Col(name) != nil }

// Names son los nombres de columnas en orden.
func (f *Frame) Names() []string {
	out := make([]string, len(f.Cols))
	for i, c := range f.Cols {
		out[i] = c.Name
	}
	return out
}

// Set agrega o reemplaza una columna.
func (f *Frame) Set(c *Column) {
	for i, old := range f.Cols {
		if old.Name == c.Name {
			f.Cols[i] = c
			return
		}
	}
	f.Cols = append(f.Cols, c)
}

// SetFloats agrega o reemplaza una columna Float.
func (f *Frame) SetFloats(name string, v []float64) { f.Set(&Column{Name: name, Kind: Float, F: v}) }

// SetInts agrega o reemplaza una columna Int.
func (f *Frame) SetInts(name string, v []int64) { f.Set(&Column{Name: name, Kind: Int, I: v}) }

// Take devuelve una tabla con las filas idx, en ese orden.
func (f *Frame) Take(idx []int) *Frame {
	out := &Frame{N: len(idx)}
	for _, c := range f.Cols {
		out.Cols = append(out.Cols, c.take(idx))
	}
	return out
}

// Select devuelve solo las columnas dadas (las que no existen se omiten), en ese orden.
func (f *Frame) Select(names []string) *Frame {
	out := &Frame{N: f.N}
	for _, n := range names {
		if c := f.Col(n); c != nil {
			out.Cols = append(out.Cols, c)
		}
	}
	return out
}

// Clone copia la estructura (las columnas se comparten; Set reemplaza sin tocar el original).
func (f *Frame) Clone() *Frame { return &Frame{Cols: append([]*Column(nil), f.Cols...), N: f.N} }

// RoundFloats redondea todas las columnas Float a d decimales (DataFrame.round).
func (f *Frame) RoundFloats(d int) *Frame {
	out := &Frame{N: f.N}
	for _, c := range f.Cols {
		if c.Kind != Float {
			out.Cols = append(out.Cols, c)
			continue
		}
		v := make([]float64, len(c.F))
		for i, x := range c.F {
			v[i] = Round(x, d)
		}
		out.Cols = append(out.Cols, &Column{Name: c.Name, Kind: Float, F: v})
	}
	return out
}

// naStrings son los textos que se leen como faltantes por defecto.
var naStrings = map[string]bool{
	"": true, "#N/A": true, "#N/A N/A": true, "#NA": true, "-1.#IND": true, "-1.#QNAN": true, "-NaN": true,
	"-nan": true, "1.#IND": true, "1.#QNAN": true, "<NA>": true, "N/A": true, "NA": true, "NULL": true,
	"NaN": true, "None": true, "n/a": true, "nan": true, "null": true,
}

var intRe = regexp.MustCompile(`^[+-]?\d+$`)

// ReadCSVFile lee un CSV con separador coma, encabezado, inferencia de tipo por columna
// (enteros sin faltantes → Int; numéricos → Float; el resto → Str).
func ReadCSVFile(path string) (*Frame, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return ReadCSV(fh)
}

// ReadCSV lee un CSV desde r (ver ReadCSVFile).
func ReadCSV(r io.Reader) (*Frame, error) { return ReadCSVSep(r, ',') }

// ReadCSVSep lee un CSV con el separador dado.
func ReadCSVSep(r io.Reader, sep rune) (*Frame, error) {
	br := bufio.NewReader(r)
	if b, err := br.Peek(3); err == nil && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		_, _ = br.Discard(3)
	}
	cr := csv.NewReader(br)
	cr.Comma = sep
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	recs, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, errors.New("CSV vacío")
	}
	header := recs[0]
	rows := recs[1:]
	f := &Frame{N: len(rows)}
	for j, name := range header {
		raw := make([]string, len(rows))
		for i, rec := range rows {
			if j < len(rec) {
				raw[i] = rec[j]
			}
		}
		f.Cols = append(f.Cols, inferColumn(name, raw))
	}
	return f, nil
}

func inferColumn(name string, raw []string) *Column {
	n := len(raw)
	nulls := 0
	allInt, allNum := true, true
	nums := make([]float64, n)
	for i, s := range raw {
		if naStrings[s] {
			nulls++
			nums[i] = NaN
			continue
		}
		t := strings.TrimSpace(s)
		v, err := strconv.ParseFloat(t, 64)
		if err != nil {
			allNum, allInt = false, false
			continue
		}
		nums[i] = v
		if !intRe.MatchString(t) {
			allInt = false
		}
	}
	switch {
	case nulls == n: // vacía: float64 NaN
		return &Column{Name: name, Kind: Float, F: nums}
	case allNum && allInt && nulls == 0:
		ints := make([]int64, n)
		for i, s := range raw {
			v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
			if err != nil { // no cabe en int64: se lee como float
				return &Column{Name: name, Kind: Float, F: nums}
			}
			ints[i] = v
		}
		return &Column{Name: name, Kind: Int, I: ints}
	case allNum:
		return &Column{Name: name, Kind: Float, F: nums}
	}
	c := &Column{Name: name, Kind: Str, S: make([]string, n), Null: make([]bool, n)}
	for i, s := range raw {
		if naStrings[s] {
			c.Null[i] = true
		} else {
			c.S[i] = s
		}
	}
	return c
}

// WriteCSV escribe la tabla como DataFrame.to_csv(index=False): coma, comillas mínimas, "\n".
func (f *Frame) WriteCSV(w io.Writer) error { return f.WriteCSVSep(w, ',') }

// WriteCSVSep es WriteCSV con otro separador.
func (f *Frame) WriteCSVSep(w io.Writer, sep byte) error {
	bw := bufio.NewWriter(w)
	names := f.Names()
	if err := writeRecord(bw, names, sep); err != nil {
		return err
	}
	rec := make([]string, len(f.Cols))
	for i := 0; i < f.N; i++ {
		for j, c := range f.Cols {
			rec[j] = c.Cell(i)
		}
		if err := writeRecord(bw, rec, sep); err != nil {
			return err
		}
	}
	return bw.Flush()
}

func writeRecord(w *bufio.Writer, rec []string, sep byte) error {
	for i, s := range rec {
		if i > 0 {
			if err := w.WriteByte(sep); err != nil {
				return err
			}
		}
		if strings.ContainsAny(s, ",\"\r\n") {
			s = `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
		}
		if _, err := w.WriteString(s); err != nil {
			return err
		}
	}
	return w.WriteByte('\n')
}

// Cells crea una columna desde valores Go heterogéneos como lo haría pd.DataFrame(list_of_dicts):
// todos int sin nil → Int; numéricos (con nil/float) → Float; con texto → Str (los números se
// imprimen con su repr); todo nil → Float NaN.
func Cells(name string, vals []any) *Column {
	allNil, anyStr, anyFloat, anyNil := true, false, false, false
	for _, v := range vals {
		switch x := v.(type) {
		case nil:
			anyNil = true
		case string:
			anyStr, allNil = true, false
		case float64:
			anyFloat, allNil = true, false
			_ = x
		case float32, int, int32, int64:
			allNil = false
		default:
			anyStr, allNil = true, false
		}
	}
	n := len(vals)
	switch {
	case allNil:
		f := make([]float64, n)
		for i := range f {
			f[i] = NaN
		}
		return &Column{Name: name, Kind: Float, F: f}
	case anyStr:
		c := &Column{Name: name, Kind: Str, S: make([]string, n), Null: make([]bool, n)}
		for i, v := range vals {
			switch x := v.(type) {
			case nil:
				c.Null[i] = true
			case string:
				c.S[i] = x
			case float64:
				if math.IsNaN(x) {
					c.Null[i] = true
				} else {
					c.S[i] = FloatRepr(x)
				}
			default:
				c.S[i] = fmt.Sprint(x)
			}
		}
		return c
	case anyFloat || anyNil:
		f := make([]float64, n)
		for i, v := range vals {
			f[i] = toFloat(v)
		}
		return &Column{Name: name, Kind: Float, F: f}
	}
	iv := make([]int64, n)
	for i, v := range vals {
		switch x := v.(type) {
		case int:
			iv[i] = int64(x)
		case int32:
			iv[i] = int64(x)
		case int64:
			iv[i] = x
		case float32:
			iv[i] = int64(x)
		}
	}
	return &Column{Name: name, Kind: Int, I: iv}
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	}
	return NaN
}

// Strs devuelve la columna como textos (los faltantes como "").
func (c *Column) Strs() []string {
	out := make([]string, c.Len())
	for i := range out {
		out[i] = c.Cell(i)
	}
	return out
}

// SetStrs agrega o reemplaza una columna de texto (sin faltantes).
func (f *Frame) SetStrs(name string, v []string) {
	f.Set(&Column{Name: name, Kind: Str, S: v, Null: make([]bool, len(v))})
}

// Filter devuelve las filas donde keep es true, en orden.
func (f *Frame) Filter(keep []bool) *Frame {
	var idx []int
	for i, k := range keep {
		if k {
			idx = append(idx, i)
		}
	}
	return f.Take(idx)
}

// DropNA descarta las filas con algún faltante en las columnas dadas (dropna(subset=…)).
func (f *Frame) DropNA(subset []string) *Frame {
	keep := make([]bool, f.N)
	for i := range keep {
		keep[i] = true
	}
	for _, n := range subset {
		c := f.Col(n)
		for i := 0; i < f.N; i++ {
			if c.IsNull(i) {
				keep[i] = false
			}
		}
	}
	return f.Filter(keep)
}

// Head devuelve las primeras n filas.
func (f *Frame) Head(n int) *Frame {
	n = min(n, f.N)
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	return f.Take(idx)
}

// Copy copia el marco (columnas incluidas) para poder modificarlo sin afectar al original.
func (f *Frame) Copy() *Frame {
	out := &Frame{N: f.N}
	for _, c := range f.Cols {
		cc := *c
		cc.F = append([]float64(nil), c.F...)
		cc.I = append([]int64(nil), c.I...)
		cc.S = append([]string(nil), c.S...)
		cc.Null = append([]bool(nil), c.Null...)
		out.Cols = append(out.Cols, &cc)
	}
	return out
}

// EmptyLike crea una tabla sin filas con las mismas columnas y tipos.
func (f *Frame) EmptyLike() *Frame { return f.Take(nil) }
