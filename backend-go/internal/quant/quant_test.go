package quant

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

type fixture struct {
	Cases []struct {
		Values []*float64          `json:"values"`
		Mean   *float64            `json:"mean"`
		Std0   *float64            `json:"std0"`
		Q      map[string]*float64 `json:"q"`
		Winsor []*float64          `json:"winsor"`
		Z      []*float64          `json:"z"`
		Rank   []*float64          `json:"rank"`
	} `json:"cases"`
	Reprs []struct {
		X    float64 `json:"x"`
		Repr string  `json:"repr"`
	} `json:"reprs"`
	Rounds []struct {
		X  float64 `json:"x"`
		R4 float64 `json:"r4"`
	} `json:"rounds"`
}

func load(t *testing.T) fixture {
	t.Helper()
	b, err := os.ReadFile("testdata/stats.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func vec(in []*float64) []float64 {
	out := make([]float64, len(in))
	for i, p := range in {
		if p == nil {
			out[i] = NaN
		} else {
			out[i] = *p
		}
	}
	return out
}

func same(a, b float64) bool {
	return (math.IsNaN(a) && math.IsNaN(b)) || math.Float64bits(a) == math.Float64bits(b)
}

func sameVec(t *testing.T, what string, got, want []float64, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: largo %d vs %d", what, len(got), len(want))
	}
	for i := range got {
		ok := same(got[i], want[i])
		if !ok && tol > 0 && !math.IsNaN(got[i]) && !math.IsNaN(want[i]) {
			ok = math.Abs(got[i]-want[i]) <= tol*math.Max(1, math.Abs(want[i]))
		}
		if !ok {
			t.Fatalf("%s[%d]: %v (obtenido) vs %v (esperado)", what, i, got[i], want[i])
		}
	}
}

func fmtSscan(s string, q *float64) (int, error) {
	v, err := parseFloat(s)
	*q = v
	return 1, err
}

func TestOrderDescEstableConNaNAlFinal(t *testing.T) {
	got := OrderDesc([]float64{1, NaN, 3, 3, 2, NaN})
	want := []int{2, 3, 4, 0, 1, 5}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v", got)
		}
	}
}

func TestFrameCSVIdaYVuelta(t *testing.T) {
	in := "Ranking,Símbolo,Nombre,Score,Cuenta,Vacía\n" +
		"1.0,A.SN,\"Uno, S.A.\",0.7148,13,\n" +
		"2.0,B.SN,Dos,,20,\n"
	f, err := ReadCSV(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	kinds := []Kind{Float, Str, Str, Float, Int, Float}
	for i, c := range f.Cols {
		if c.Kind != kinds[i] {
			t.Errorf("columna %s: tipo %d, esperaba %d", c.Name, c.Kind, kinds[i])
		}
	}
	var sb strings.Builder
	if err := f.WriteCSV(&sb); err != nil {
		t.Fatal(err)
	}
	if sb.String() != in {
		t.Fatalf("no es idéntico:\n%q\n%q", sb.String(), in)
	}
}

func TestCellsComoDataFrame(t *testing.T) {
	cases := []struct {
		vals []any
		kind Kind
		cell string
	}{
		{[]any{1, 2}, Int, "1"},
		{[]any{1, nil}, Float, "1.0"},
		{[]any{1.5, nil}, Float, "1.5"},
		{[]any{nil, nil}, Float, ""},
		{[]any{"a", 2.0}, Str, "a"},
		{[]any{2.0, "a"}, Str, "2.0"},
	}
	for i, c := range cases {
		col := Cells("x", c.vals)
		if col.Kind != c.kind || col.Cell(0) != c.cell {
			t.Errorf("caso %d: tipo %d celda %q", i, col.Kind, col.Cell(0))
		}
	}
}

func parseFloat(s string) (float64, error) { return strconv.ParseFloat(s, 64) }
