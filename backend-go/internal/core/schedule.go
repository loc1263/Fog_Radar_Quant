package core

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

var diasES = [...]string{"lunes", "martes", "miércoles", "jueves", "viernes", "sábado", "domingo"}

// Programa es una corrida programada del cron de producción.
type Programa struct {
	Key     string
	Titulo  string
	Modulos []string
	// Dias: 0 = lunes … 6 = domingo.
	Dias   []int
	Hora   int
	Minuto int
}

// Programas es la única fuente de verdad de la programación.
var Programas = []Programa{
	{Key: "semanal", Titulo: "ETFs", Modulos: []string{"etf"}, Dias: []int{0}, Hora: 6},
	{Key: "diario", Titulo: "Chile, EEUU y Tendencia", Modulos: []string{"cl_stocks", "us_stocks", "tendencia"},
		Dias: []int{0, 1, 2, 3, 4}, Hora: 18},
}

// weekdayMon0: lunes = 0 … domingo = 6.
func weekdayMon0(t time.Time) int { return (int(t.Weekday()) + 6) % 7 }

// Cron es la expresión cron (en cron el lunes es 1 y el domingo 0/7).
func (p Programa) Cron() string {
	dias := make([]int, len(p.Dias))
	for i, d := range p.Dias {
		dias[i] = d + 1
	}
	slices.Sort(dias)
	var campo string
	consecutivos := len(dias) > 1 && dias[len(dias)-1]-dias[0] == len(dias)-1
	if consecutivos {
		campo = fmt.Sprintf("%d-%d", dias[0], dias[len(dias)-1])
	} else {
		s := make([]string, len(dias))
		for i, d := range dias {
			s[i] = fmt.Sprint(d)
		}
		campo = strings.Join(s, ",")
	}
	return fmt.Sprintf("%d %d * * %s", p.Minuto, p.Hora, campo)
}

// Descripcion es el texto «de lunes a viernes a las 18:00».
func (p Programa) Descripcion() string {
	var cuando string
	switch {
	case slices.Equal(p.Dias, []int{0, 1, 2, 3, 4}):
		cuando = "de lunes a viernes"
	case len(p.Dias) == 1:
		cuando = "los " + diasES[p.Dias[0]]
	default:
		n := make([]string, len(p.Dias))
		for i, d := range p.Dias {
			n[i] = diasES[d]
		}
		cuando = strings.Join(n, ", ")
	}
	return fmt.Sprintf("%s a las %02d:%02d", cuando, p.Hora, p.Minuto)
}

// Proxima es la próxima ejecución estrictamente posterior a ahora (en su zona).
func (p Programa) Proxima(ahora time.Time) time.Time {
	y, m, d := ahora.Date()
	for delta := 0; delta < 8; delta++ {
		c := time.Date(y, m, d+delta, p.Hora, p.Minuto, 0, 0, ahora.Location())
		if slices.Contains(p.Dias, weekdayMon0(c)) && c.After(ahora) {
			return c
		}
	}
	return ahora
}

// Anterior es la última ejecución programada en o antes de ahora (en su zona).
func (p Programa) Anterior(ahora time.Time) time.Time {
	y, m, d := ahora.Date()
	for delta := 0; delta < 8; delta++ {
		c := time.Date(y, m, d-delta, p.Hora, p.Minuto, 0, 0, ahora.Location())
		if slices.Contains(p.Dias, weekdayMon0(c)) && !c.After(ahora) {
			return c
		}
	}
	return ahora
}

// ProgramaDe devuelve el programa que corre un módulo, si lo hay.
func ProgramaDe(modulo string) (Programa, bool) {
	for _, p := range Programas {
		if slices.Contains(p.Modulos, modulo) {
			return p, true
		}
	}
	return Programa{}, false
}
