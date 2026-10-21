package tend

import (
	"math"
	"strconv"
	"testing"
	"time"

	"fogradar/internal/quant"
)

// preciosMensuales arma 14 velas mensuales: A sube 5 % por mes, B se mantiene plano y C cae 3 % por mes.
func preciosMensuales() *Prices {
	p := &Prices{Tickers: []string{"A", "B", "C"}, Cols: make([][]float64, 3)}
	for i := range 14 {
		p.Dates = append(p.Dates, time.Date(2024, time.Month(1+i), 28, 0, 0, 0, 0, time.UTC))
		p.Cols[0] = append(p.Cols[0], 100*math.Pow(1.05, float64(i)))
		p.Cols[1] = append(p.Cols[1], 100)
		p.Cols[2] = append(p.Cols[2], 100*math.Pow(0.97, float64(i)))
	}
	return p
}

func TestMomentumTableVentanasMensuales(t *testing.T) {
	f := MomentumTable(preciosMensuales(), "mensual", &Universe{})
	if f.N != 3 {
		t.Fatalf("filas = %d, quiero 3", f.N)
	}
	for _, w := range []int{1, 2, 4, 6, 8, 10, 12} {
		name := "momentum_" + strconv.Itoa(w) + "m_pct"
		if f.Col(name) == nil {
			t.Fatalf("falta la columna %s", name)
		}
	}
	if f.Col("momentum_3m_pct") != nil {
		t.Errorf("momentum_3m_pct ya no es una ventana del ranking mensual")
	}
	if got := f.Cols[0].Cell(0); got != "1" {
		t.Errorf("rank de la primera fila = %q, quiero 1", got)
	}
	if got := f.Col("ticker").Cell(0); got != "A" {
		t.Errorf("primer lugar = %q, quiero A (el único que sube)", got)
	}
	if got := f.Col("ticker").Cell(2); got != "C" {
		t.Errorf("último lugar = %q, quiero C (el que cae)", got)
	}
	want := roundExact((math.Pow(1.05, 12)-1)*100, 2)
	if got := f.Col("momentum_12m_pct").At(0); math.Abs(got-want) > 1e-9 {
		t.Errorf("momentum_12m_pct de A = %v, quiero %v", got, want)
	}
	want4 := roundExact((math.Pow(1.05, 4)-1)*100, 2)
	if got := f.Col("momentum_4m_pct").At(0); math.Abs(got-want4) > 1e-9 {
		t.Errorf("momentum_4m_pct de A = %v, quiero %v", got, want4)
	}
}

func TestMomentumTableHistoriaMinima(t *testing.T) {
	p := preciosMensuales()
	corto := &Prices{Dates: p.Dates[:12], Tickers: p.Tickers, Cols: [][]float64{p.Cols[0][:12], p.Cols[1][:12], p.Cols[2][:12]}}
	if f := MomentumTable(corto, "mensual", &Universe{}); f.N != 0 {
		t.Errorf("con 12 meses no debe rankear a nadie (mínimo 13), filas = %d", f.N)
	}
}

func TestMomentumScorePromediaTodasLasVentanas(t *testing.T) {
	m := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7}
	got := momentumScore(m, 1)
	if math.Abs(got-0.4) > 1e-12 {
		t.Errorf("score con consistencia 1 = %v, quiero 0.4", got)
	}
	// consistencia 0 reduce el score a la mitad; consistencia ausente cuenta como 0.5.
	if got := momentumScore(m, 0); math.Abs(got-0.2) > 1e-12 {
		t.Errorf("score con consistencia 0 = %v, quiero 0.2", got)
	}
	if got := momentumScore(m, math.NaN()); math.Abs(got-0.3) > 1e-12 {
		t.Errorf("score con consistencia NaN = %v, quiero 0.3", got)
	}
	m[3] = math.NaN()
	if got := momentumScore(m, 1); !math.IsNaN(got) {
		t.Errorf("si falta una ventana el score es NaN, fue %v", got)
	}
	if got := momentumScore(nil, 1); !math.IsNaN(got) {
		t.Errorf("sin ventanas el score es NaN, fue %v", got)
	}
}

// preciosSemanales arma 20 velas semanales (viernes): A sube 2 % por semana, B plano, C cae 1 % por semana y
// D solo tiene las últimas 10 velas.
func preciosSemanales() *Prices {
	p := &Prices{Tickers: []string{"A", "B", "C", "D"}, Cols: make([][]float64, 4)}
	for i := range 20 {
		p.Dates = append(p.Dates, time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 7*i))
		p.Cols[0] = append(p.Cols[0], 100*math.Pow(1.02, float64(i)))
		p.Cols[1] = append(p.Cols[1], 100)
		p.Cols[2] = append(p.Cols[2], 100*math.Pow(0.99, float64(i)))
		d := quant.NaN
		if i >= 10 {
			d = 50 + float64(i)
		}
		p.Cols[3] = append(p.Cols[3], d)
	}
	return p
}

func TestRetornoTableOrdenaPorRetornoPuro(t *testing.T) {
	f := RetornoTable(preciosSemanales(), 17, &Universe{})
	if f.N != 3 {
		t.Fatalf("filas = %d, quiero 3 (D no tiene 18 velas)", f.N)
	}
	for i, want := range []string{"A", "B", "C"} {
		if got := f.Col("ticker").Cell(i); got != want {
			t.Errorf("puesto %d = %q, quiero %q", i+1, got, want)
		}
	}
	want := roundExact((math.Pow(1.02, 17)-1)*100, 2)
	if got := f.Col("retorno_pct").At(0); math.Abs(got-want) > 1e-9 {
		t.Errorf("retorno_pct de A = %v, quiero %v", got, want)
	}
	if got := f.Col("retorno_pct").At(2); got >= 0 {
		t.Errorf("retorno_pct de C = %v, debe ser negativo", got)
	}
}

func TestRetornoTableSinHistoriaSuficiente(t *testing.T) {
	if f := RetornoTable(preciosSemanales(), 25, &Universe{}); f.N != 0 {
		t.Errorf("con 25 semanas pedidas y 20 velas no debe rankear a nadie, filas = %d", f.N)
	}
}

func TestParamsRetornoReciente(t *testing.T) {
	p := DefaultParams()
	if p.TopNRetorno != 8 || p.RetornoWeeks != 17 {
		t.Errorf("TopNRetorno, RetornoWeeks = %d, %d; quiero 8, 17", p.TopNRetorno, p.RetornoWeeks)
	}
	if td := p.Td(); td.TopNRetorno != 8 || td.RetornoSemanas != 17 {
		t.Errorf("Td() = %d, %d; quiero 8, 17", td.TopNRetorno, td.RetornoSemanas)
	}
}
