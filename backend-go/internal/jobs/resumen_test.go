package jobs

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"fogradar/internal/yahoo"
)

func TestMiles(t *testing.T) {
	for n, want := range map[int64]string{0: "0", 93: "93", 999: "999", 1000: "1.000", 2193: "2.193", 1234567: "1.234.567"} {
		if got := miles(n); got != want {
			t.Errorf("miles(%d) = %q, quería %q", n, got, want)
		}
	}
}

func TestResumenEnTablaAlineada(t *testing.T) {
	t0 := time.Date(2026, 10, 20, 23, 34, 46, 0, time.UTC)
	out := Resumen([]Resultado{
		{Modulo: "cl_stocks", Estado: EstadoOK, Duracion: 19 * time.Second, Detalle: "30/30 tickers · top 3: CHILE.SN",
			Mensaje: "30/30 tickers · 93 requests a Yahoo", Stats: yahoo.Stats{Requests: 93, Interval: 200 * time.Millisecond},
			Inicio: t0, Fin: t0.Add(19 * time.Second)},
		{Modulo: "etf", Estado: EstadoOK, Duracion: 439 * time.Second, Detalle: "769/1226 ETFs depurados",
			Stats:  yahoo.Stats{Requests: 2193, RateLimited: 2, Interval: 400 * time.Millisecond, Widened: 1},
			Inicio: t0.Add(time.Minute), Fin: t0.Add(8*time.Minute + 19*time.Second)},
		{Modulo: "us_stocks", Estado: EstadoOmitido, Mensaje: "sin modo offline"},
	})
	lineas := strings.Split(out, "\n")
	if lineas[0] != "Resumen de la corrida" {
		t.Errorf("título: %q", lineas[0])
	}
	// Las columnas quedan alineadas: «Requests» termina en la misma posición en el encabezado y en cada fila.
	fin := func(l, palabra string) int { // posición en caracteres (la «ó» de «Módulo» ocupa 2 bytes)
		return utf8.RuneCountInString(l[:strings.Index(l, palabra)]) + utf8.RuneCountInString(palabra)
	}
	if a, b := fin(lineas[1], "Requests"), fin(lineas[2], "93"); a != b {
		t.Errorf("columna Requests desalineada: encabezado %d, fila %d\n%s", a, b, out)
	}
	if a, b := fin(lineas[1], "Requests"), fin(lineas[3], "2.193"); a != b {
		t.Errorf("columna Requests desalineada: encabezado %d, fila %d\n%s", a, b, out)
	}
	for _, want := range []string{
		"cl_stocks  OK", "00:00:19", "93", "0.20 s",
		"etf        OK", "00:07:19", "2.193", "0.40 s (ensanchado 1 veces)",
		"us_stocks  omitido",
		"Tiempo total: 00:08:19",
		"Resultados", "cl_stocks  30/30 tickers · top 3: CHILE.SN", "etf        769/1226 ETFs depurados",
		"Sin avisos.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q en:\n%s", want, out)
		}
	}
	if strings.Contains(out, "requests a Yahoo") {
		t.Errorf("«Resultados» no debe repetir el resumen de tráfico que ya está en la tabla:\n%s", out)
	}
	if !strings.Contains(out, "us_stocks  sin modo offline") {
		t.Errorf("un módulo omitido muestra su motivo:\n%s", out)
	}
}

func TestResumenMarcaLosErrores(t *testing.T) {
	out := Resumen([]Resultado{{Modulo: "etf", Estado: EstadoError, Mensaje: "boom", Duracion: time.Second}})
	if !strings.Contains(out, "etf      ERROR ") || !strings.Contains(out, "etf      ERROR: boom") {
		t.Errorf("\n%s", out)
	}
}

func TestEnvolverAvisoLargo(t *testing.T) {
	msg := "[EEUU] excluidas 30 empresas con sede fuera de EEUU (ADR): " + strings.Repeat("TSM (Taiwan), ", 20) + "BHP (Australia)"
	out := EnvolverAviso("    · ", msg)
	ls := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(ls) < 3 {
		t.Fatalf("debía partirse en varias líneas:\n%s", out)
	}
	if !strings.HasPrefix(ls[0], "    · [EEUU] excluidas 30 empresas") || !strings.HasSuffix(ls[0], "(ADR):") {
		t.Errorf("el encabezado va solo en la primera línea: %q", ls[0])
	}
	for i, l := range ls {
		if n := len([]rune(l)); n > anchoAviso+8 {
			t.Errorf("línea %d de %d caracteres (máx ≈ %d): %q", i, n, anchoAviso, l)
		}
		if i > 0 && !strings.HasPrefix(l, "        ") {
			t.Errorf("las continuaciones van sangradas: %q", l)
		}
	}
	if !strings.Contains(strings.Join(ls, " "), "BHP (Australia)") {
		t.Error("no debe perderse texto al partir")
	}
	if got := EnvolverAviso("    · ", "corto"); got != "    · corto\n" {
		t.Errorf("un aviso corto queda en una línea: %q", got)
	}
}
