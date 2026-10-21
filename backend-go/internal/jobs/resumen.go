package jobs

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Resumen formatea el cierre de una secuencia para la consola: una tabla alineada con el tiempo (HH:MM:SS) y el
// tráfico a Yahoo de cada módulo, el resultado de cada uno y los avisos (WARN/ERROR), con el ticker cuando el aviso
// es de uno. El detalle completo queda en data/_jobs/logs/fogradar.log.
func Resumen(res []Resultado) string {
	var b strings.Builder
	b.WriteString("Resumen de la corrida\n")

	ancho := len("Módulo")
	for _, r := range res {
		ancho = max(ancho, utf8.RuneCountInString(r.Modulo))
	}
	fmt.Fprintf(&b, "  %-*s  %-7s  %-8s  %9s  %4s  %s\n", ancho, "Módulo", "Estado", "Tiempo", "Requests", "429", "Espaciado")
	var ini, fin time.Time
	for _, r := range res {
		estado := map[string]string{EstadoOK: "OK", EstadoError: "ERROR", EstadoOmitido: "omitido"}[r.Estado]
		requests, limitados, espaciado := "-", "-", "-"
		if r.Stats.Requests > 0 {
			requests, limitados = miles(r.Stats.Requests), fmt.Sprint(r.Stats.RateLimited)
			espaciado = "fijo"
			if r.Stats.Interval > 0 {
				espaciado = fmt.Sprintf("%.2f s", r.Stats.Interval.Seconds())
				if r.Stats.Widened > 0 {
					espaciado += fmt.Sprintf(" (ensanchado %d veces)", r.Stats.Widened)
				}
			}
		}
		tiempo := HMS(r.Duracion)
		if r.Estado == EstadoOmitido {
			tiempo = "-"
		}
		fmt.Fprintf(&b, "  %-*s  %-7s  %-8s  %9s  %4s  %s\n", ancho, r.Modulo, estado, tiempo, requests, limitados, espaciado)
		if r.Inicio.IsZero() {
			continue
		}
		if ini.IsZero() || r.Inicio.Before(ini) {
			ini = r.Inicio
		}
		if r.Fin.After(fin) {
			fin = r.Fin
		}
	}
	if !ini.IsZero() {
		fmt.Fprintf(&b, "  Tiempo total: %s  (%s → %s)\n", HMS(fin.Sub(ini)), ini.Format("15:04:05"), fin.Format("15:04:05"))
	}

	b.WriteString("\nResultados\n")
	for _, r := range res {
		texto := r.Detalle
		if texto == "" && r.Estado != EstadoOK { // en un OK el Mensaje trae el resumen de tráfico, ya está en la tabla
			texto = r.Mensaje
		}
		if texto == "" {
			continue
		}
		if r.Estado == EstadoError {
			texto = "ERROR: " + texto
		}
		fmt.Fprintf(&b, "  %-*s  %s\n", ancho, r.Modulo, texto)
	}

	if av := seccionAvisos(res); av != "" {
		b.WriteString("\nAvisos\n" + av)
	} else {
		b.WriteString("\nSin avisos.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// miles escribe un entero con punto como separador de miles (2193 → «2.193»).
func miles(n int64) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return s
}

// maxAvisosPorModulo limita las líneas de avisos de un módulo en el resumen (el detalle completo queda en fogradar.log).
const maxAvisosPorModulo = 40

// anchoAviso es el ancho de línea (en caracteres) de un aviso largo en el resumen.
const anchoAviso = 96

// seccionAvisos arma los avisos del resumen: por módulo, primero los de tickers y luego los generales.
func seccionAvisos(res []Resultado) string {
	var b strings.Builder
	for _, r := range res {
		if len(r.Avisos) == 0 {
			continue
		}
		fmt.Fprintf(&b, "  %s (%d)\n", r.Modulo, len(r.Avisos))
		var tickers, otros []Aviso
		for _, a := range r.Avisos {
			if a.Ticker != "" {
				tickers = append(tickers, a)
			} else {
				otros = append(otros, a)
			}
		}
		n := 0
		linea := func(s string) {
			if n < maxAvisosPorModulo {
				b.WriteString(s)
			}
			n++
		}
		for _, a := range tickers {
			linea(fmt.Sprintf("    %-12s %s\n", a.Ticker, a.Mensaje))
		}
		for _, a := range otros {
			linea(envolverAviso("    · ", a.Mensaje))
		}
		if n > maxAvisosPorModulo {
			fmt.Fprintf(&b, "    … y %d más (ver data/_jobs/logs/fogradar.log)\n", n-maxAvisosPorModulo)
		}
	}
	return b.String()
}

// envolverAviso parte un aviso largo en varias líneas: el encabezado (hasta los primeros «: ») en la primera y la
// lista que sigue repartida en líneas de anchoAviso cortando después de las comas. prefijo encabeza la primera línea.
func envolverAviso(prefijo, msg string) string {
	sangria := strings.Repeat(" ", utf8.RuneCountInString(prefijo)+2)
	if utf8.RuneCountInString(msg) <= anchoAviso {
		return prefijo + msg + "\n"
	}
	cabeza, cola := "", msg
	if i := strings.Index(msg, ": "); i > 0 {
		cabeza, cola = msg[:i+1], msg[i+2:]
	}
	var b strings.Builder
	if cabeza != "" {
		b.WriteString(prefijo + cabeza + "\n")
	} else {
		sangria = strings.Repeat(" ", utf8.RuneCountInString(prefijo))
	}
	linea := ""
	vaciar := func() {
		if linea != "" {
			if cabeza == "" && b.Len() == 0 {
				b.WriteString(prefijo + strings.TrimRight(linea, " ") + "\n")
			} else {
				b.WriteString(sangria + strings.TrimRight(linea, " ") + "\n")
			}
			linea = ""
		}
	}
	for _, trozo := range strings.SplitAfter(cola, ", ") {
		if linea != "" && utf8.RuneCountInString(sangria)+utf8.RuneCountInString(linea)+utf8.RuneCountInString(trozo) > anchoAviso {
			vaciar()
		}
		linea += trozo
	}
	vaciar()
	return b.String()
}
