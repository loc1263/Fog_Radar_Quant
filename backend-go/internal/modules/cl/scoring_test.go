package cl

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"fogradar/internal/quant"
)

func norm(b []byte) string { return strings.ReplaceAll(string(b), "\r\n", "\n") }

// Paridad con Stock_Choice_Nacional: mismo CSV de indicadores → mismo ranking, byte a byte.
func TestRankingIdenticoAlOriginal(t *testing.T) {
	for _, ts := range []string{"20260924_221437", "20260925_183129"} {
		ind, err := quant.ReadCSVFile("testdata/indicadores_acciones_" + ts + ".csv")
		if err != nil {
			t.Fatal(err)
		}
		rank, err := ConstruirRanking(ind, nil)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := rank.WriteCSV(&buf); err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile("testdata/ranking_acciones_" + ts + ".csv")
		if err != nil {
			t.Fatal(err)
		}
		if got := buf.String(); got != norm(want) {
			gl, wl := strings.Split(got, "\n"), strings.Split(norm(want), "\n")
			for i := range wl {
				if i >= len(gl) || gl[i] != wl[i] {
					t.Fatalf("%s: difiere en la línea %d\n go: %s\norig: %s", ts, i+1, at(gl, i), wl[i])
				}
			}
			t.Fatalf("%s: difiere en el largo", ts)
		}
	}
}

func at(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "<fin>"
}

func TestFaltanColumnasDeIdentidad(t *testing.T) {
	f := quant.NewFrame(1)
	f.SetFloats("Símbolo", []float64{1})
	if _, err := ConstruirRanking(f, nil); err == nil || !strings.Contains(err.Error(), "Nombre corto") {
		t.Fatal(err)
	}
}

func TestDuplicadosFinancieros(t *testing.T) {
	f := quant.NewFrame(4)
	f.SetFloats("Revenue (ttm)", []float64{100, 100.05, 500, quant.NaN})
	f.SetFloats("EBITDA", []float64{10, 10, 50, quant.NaN})
	g := DetectarDuplicados(f)
	if g[0] != 1 || g[1] != 1 || g[2] != 0 || g[3] != 0 {
		t.Fatalf("%v", g)
	}
}
