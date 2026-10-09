package tend

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"fogradar/internal/quant"
)

// KV es un par clave-valor de un objeto JSON ordenado.
type KV struct {
	K string
	V any
}

// OM es un objeto JSON que conserva el orden de inserción.
type OM []KV

// OrderedJSON serializa JSON con claves en orden de inserción: sangría de 2 espacios, floats con repr,
// una línea por elemento de lista. Acepta OM, []any, []string, []float64, string, bool, nil, int*, float64.
func OrderedJSON(v any) []byte {
	var b bytes.Buffer
	writeJSON(&b, v, 0)
	return b.Bytes()
}

func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func writeJSON(b *bytes.Buffer, v any, depth int) {
	ind := strings.Repeat("  ", depth+1)
	end := strings.Repeat("  ", depth)
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case string:
		b.WriteString(jsonString(x))
	case int:
		b.WriteString(strconv.Itoa(x))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case float64:
		switch {
		case math.IsNaN(x):
			b.WriteString("NaN")
		case math.IsInf(x, 1):
			b.WriteString("Infinity")
		case math.IsInf(x, -1):
			b.WriteString("-Infinity")
		default:
			b.WriteString(quant.FloatRepr(x))
		}
	case []string:
		a := make([]any, len(x))
		for i, s := range x {
			a[i] = s
		}
		writeJSON(b, a, depth)
	case []float64:
		a := make([]any, len(x))
		for i, s := range x {
			a[i] = s
		}
		writeJSON(b, a, depth)
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for i, e := range x {
			b.WriteString(ind)
			writeJSON(b, e, depth+1)
			if i < len(x)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString(end + "]")
	case OM:
		if len(x) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		for i, kv := range x {
			b.WriteString(ind + jsonString(kv.K) + ": ")
			writeJSON(b, kv.V, depth+1)
			if i < len(x)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString(end + "}")
	default:
		panic(fmt.Sprintf("OrderedJSON: tipo no soportado %T", v))
	}
}
