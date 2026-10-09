package cl

import (
	"fmt"
	"math"

	"fogradar/internal/quant"
)

// Columna es una entrada del catálogo de indicadores crudos (CSV de indicadores).
type Columna struct {
	CSV, Key, Label, Grupo, Unidad string
	Descripcion                    *string
}

func strp(s string) *string { return &s }

// Parámetros del winsorizado del universo.
const (
	WinsorInferior = 0.05
	WinsorSuperior = 0.95
)

// Metric es una métrica puntuable (scoring.py: Metric).
type Metric struct {
	Columna       string
	Pilar         string
	Invertir      bool // «menor es mejor» (múltiplos, deuda): se invierte el z-score
	SoloPositivos bool // los valores <= 0 no tienen lectura económica y se excluyen
	DivSostenible bool // solo vale si 0 < Payout Ratio <= 1
}

// Pilares en orden.
var Pilares = []string{"Valor", "Calidad", "Crecimiento", "Salud Financiera", "Dividendo", "Momentum"}

// Metrics en el orden del original.
var Metrics = []Metric{
	{Columna: "Trailing P/E", Pilar: "Valor", Invertir: true, SoloPositivos: true},
	{Columna: "Price/Book", Pilar: "Valor", Invertir: true, SoloPositivos: true},
	{Columna: "Price/Sales", Pilar: "Valor", Invertir: true, SoloPositivos: true},
	{Columna: "Enterprise Value/EBITDA", Pilar: "Valor", Invertir: true, SoloPositivos: true},
	{Columna: "Enterprise Value/Revenue", Pilar: "Valor", Invertir: true, SoloPositivos: true},
	{Columna: "Profit Margin", Pilar: "Calidad"},
	{Columna: "Operating Margin (ttm)", Pilar: "Calidad"},
	{Columna: "Return on Assets (ttm)", Pilar: "Calidad"},
	{Columna: "Return on Equity (ttm)", Pilar: "Calidad"},
	{Columna: "Quarterly Revenue Growth (yoy)", Pilar: "Crecimiento"},
	{Columna: "Quarterly Earnings Growth (yoy)", Pilar: "Crecimiento"},
	{Columna: "Total Debt/Equity (mrq)", Pilar: "Salud Financiera", Invertir: true, SoloPositivos: true},
	{Columna: "Current Ratio (mrq)", Pilar: "Salud Financiera"},
	{Columna: "Forward Annual Dividend Yield (%)", Pilar: "Dividendo", DivSostenible: true},
	{Columna: "Retorno 5D", Pilar: "Momentum"},
	{Columna: "Retorno 1M", Pilar: "Momentum"},
	{Columna: "Retorno 3M", Pilar: "Momentum"},
	{Columna: "Retorno 6M", Pilar: "Momentum"},
	{Columna: "Retorno YTD", Pilar: "Momentum"},
	{Columna: "Retorno 1A", Pilar: "Momentum"},
}

// PesosPilares son los pesos del Score Compuesto (parámetro de estilo: inclina hacia Momentum).
var PesosPilares = map[string]float64{
	"Valor": 0.5, "Calidad": 1.0, "Crecimiento": 1.0, "Salud Financiera": 1.0, "Dividendo": 0.25, "Momentum": 2.0,
}

// ColumnasIdentidad: mínimas para poder puntuar un CSV de indicadores.
var ColumnasIdentidad = []string{"Símbolo", "Nombre corto", "Sector", "Industria"}

// ColumnasRanking es el orden de columnas de ranking_acciones_*.csv.
var ColumnasRanking = func() []string {
	cols := []string{"Ranking"}
	cols = append(cols, ColumnasIdentidad...)
	cols = append(cols, "Score Compuesto")
	for _, p := range Pilares {
		cols = append(cols, "Score "+p)
	}
	cols = append(cols, "Métricas Disponibles", "Métricas Totales", "Grupo Financiero Duplicado")
	for _, m := range Metrics {
		cols = append(cols, m.Columna)
	}
	return cols
}()

func casiIguales(a, b []float64, tol float64) bool {
	for i := range a {
		if math.IsNaN(a[i]) || math.IsNaN(b[i]) {
			return false
		}
		base := math.Max(math.Max(math.Abs(a[i]), math.Abs(b[i])), 1.0)
		if math.Abs(a[i]-b[i])/base > tol {
			return false
		}
	}
	return true
}

// DetectarDuplicados da un id de grupo por fila para tickers con Revenue y EBITDA prácticamente
// idénticos (holding y filial); 0 = sin par.
func DetectarDuplicados(f *quant.Frame) []int {
	grupo := make([]int, f.N)
	cols := []string{"Revenue (ttm)", "EBITDA"}
	vals := make([][]float64, len(cols))
	for j, name := range cols {
		c := f.Col(name)
		if c == nil {
			return grupo
		}
		vals[j] = c.Floats()
	}
	row := func(i int) []float64 { return []float64{vals[0][i], vals[1][i]} }
	siguiente := 1
	for i := 0; i < f.N; i++ {
		for j := i + 1; j < f.N; j++ {
			if !casiIguales(row(i), row(j), 0.001) {
				continue
			}
			switch {
			case grupo[i] == 0 && grupo[j] == 0:
				grupo[i], grupo[j] = siguiente, siguiente
				siguiente++
			case grupo[i] != 0:
				grupo[j] = grupo[i]
			default:
				grupo[i] = grupo[j]
			}
		}
	}
	return grupo
}

func serieMetrica(f *quant.Frame, m Metric) []float64 {
	s := f.Col(m.Columna).Floats()
	if m.SoloPositivos {
		for i, v := range s {
			if !(v > 0) {
				s[i] = quant.NaN
			}
		}
	}
	// Sin la columna «Payout Ratio» el original no aplica el filtro (to_numeric(None) falla y se registra).
	if p := f.Col("Payout Ratio"); m.DivSostenible && p != nil {
		pr := p.Floats()
		for i := range s {
			if !(pr[i] > 0 && pr[i] <= 1) {
				s[i] = quant.NaN
			}
		}
	}
	return s
}

// Scores agrega z-scores, scores por pilar, Score Compuesto y Ranking, ordenado por score.
// pesos nil = PesosPilares.
func Scores(in *quant.Frame, pesos map[string]float64) *quant.Frame {
	if pesos == nil {
		pesos = PesosPilares
	}
	df := in.Clone()
	grupo := DetectarDuplicados(df)
	gv := make([]any, len(grupo))
	for i, g := range grupo {
		if g != 0 {
			gv[i] = g
		}
	}
	df.Set(quant.Cells("Grupo Financiero Duplicado", gv))

	zPorPilar := map[string][][]float64{}
	var pilaresPresentes []string
	var zTodas [][]float64
	for _, m := range Metrics {
		if !df.Has(m.Columna) {
			continue
		}
		z := quant.ZScore(quant.Winsorize(serieMetrica(df, m), WinsorInferior, WinsorSuperior))
		if m.Invertir {
			for i := range z {
				z[i] = -z[i]
			}
		}
		if _, ok := zPorPilar[m.Pilar]; !ok {
			pilaresPresentes = append(pilaresPresentes, m.Pilar)
		}
		zPorPilar[m.Pilar] = append(zPorPilar[m.Pilar], z)
		zTodas = append(zTodas, z)
	}

	n := df.N
	pilarVals := make([][]float64, len(pilaresPresentes))
	for k, p := range pilaresPresentes {
		col := make([]float64, n)
		for i := 0; i < n; i++ {
			sum, cnt := 0.0, 0
			for _, z := range zPorPilar[p] {
				if !math.IsNaN(z[i]) {
					sum += z[i]
					cnt++
				}
			}
			col[i] = quant.NaN
			if cnt > 0 {
				col[i] = sum / float64(cnt)
			}
		}
		pilarVals[k] = col
		df.SetFloats("Score "+p, col)
	}

	disp := make([]int64, n)
	for i := 0; i < n; i++ {
		for _, z := range zTodas {
			if !math.IsNaN(z[i]) {
				disp[i]++
			}
		}
	}
	tot := make([]int64, n)
	for i := range tot {
		tot[i] = int64(len(zTodas))
	}
	df.SetInts("Métricas Disponibles", disp)
	df.SetInts("Métricas Totales", tot)

	score := make([]float64, n)
	for i := 0; i < n; i++ {
		num, den := 0.0, 0.0
		for k, p := range pilaresPresentes {
			w := 1.0
			if pw, ok := pesos[p]; ok {
				w = pw
			}
			if v := pilarVals[k][i]; !math.IsNaN(v) {
				num += v * w
				den += w
			}
		}
		score[i] = quant.NaN
		if den > 0 {
			score[i] = num / den
		}
	}
	df.SetFloats("Score Compuesto", score)
	df.SetFloats("Ranking", quant.RankMinDesc(score))
	return df.Take(quant.OrderDesc(score))
}

// ConstruirRanking arma la tabla final ranking_acciones_*.csv: columnas del original, 4 decimales.
func ConstruirRanking(indicadores *quant.Frame, pesos map[string]float64) (*quant.Frame, error) {
	for _, c := range ColumnasIdentidad {
		if !indicadores.Has(c) {
			var falt []string
			for _, c2 := range ColumnasIdentidad {
				if !indicadores.Has(c2) {
					falt = append(falt, c2)
				}
			}
			return nil, fmt.Errorf("Faltan columnas en el CSV de indicadores: %s", join(falt))
		}
	}
	return Scores(indicadores, pesos).Select(ColumnasRanking).RoundFloats(4), nil
}

func join(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}

// GruposDuplicados devuelve {grupo: [símbolos]} desde la columna «Grupo Financiero Duplicado».
func GruposDuplicados(rank *quant.Frame) map[int][]string {
	out := map[int][]string{}
	g, s := rank.Col("Grupo Financiero Duplicado"), rank.Col("Símbolo")
	if g == nil || s == nil {
		return out
	}
	for i := 0; i < rank.N; i++ {
		if g.IsNull(i) {
			continue
		}
		gid := int(g.Floats()[i])
		out[gid] = append(out[gid], s.Cell(i))
	}
	return out
}
