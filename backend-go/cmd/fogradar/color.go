package main

import (
	"io"
	"os"
	"regexp"
	"strings"
)

// Colores ANSI de la consola de `fogradar job`. Solo se usan cuando la salida es una terminal (o con FORCE_COLOR) y
// no está NO_COLOR definido; en un archivo, un pipe o un test la salida queda en texto plano.
const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[90m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiCyan    = "\033[36m"
	ansiMagenta = "\033[35m"
	ansiBoldRed = "\033[1;31m"
	ansiBoldYel = "\033[1;33m"
	ansiBoldGrn = "\033[1;32m"
	ansiBoldMag = "\033[1;35m"
)

// pintar envuelve s con el código ANSI; sin color devuelve s tal cual.
func pintar(color bool, codigo, s string) string {
	if !color || s == "" {
		return s
	}
	return codigo + s + ansiReset
}

// colorActivo dice si se debe colorear lo que se escribe en w.
func colorActivo(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		habilitarColorTerminal(w) // si es una consola de Windows, activa ANSI; si es un archivo, no hace nada
		return true
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	return habilitarColorTerminal(w)
}

var (
	barraRE   = regexp.MustCompile(`^(.*?\s)\[(#*)(-*)\]\s+(\d+%)(\s+\d+/\d+)$`)
	claveRE   = regexp.MustCompile(`(\s{2})([a-z_áéíóúñ0-9]+)=`)
	estadoRE  = regexp.MustCompile(`^(  \S+\s+)(OK|ERROR|omitido)(\s|$)`)
	resultaRE = regexp.MustCompile(`^(  )(\S+)(\s+)(.*)$`)
)

// decorarInfo colorea una línea INFO de la consola: barras de avance y las claves de «clave=valor».
func decorarInfo(texto string, color bool) string {
	if !color {
		return texto
	}
	if m := barraRE.FindStringSubmatch(texto); m != nil {
		return m[1] + ansiDim + "[" + ansiReset + ansiGreen + m[2] + ansiReset + ansiDim + m[3] + "]" + ansiReset +
			" " + ansiBold + m[4] + ansiReset + ansiDim + m[5] + ansiReset
	}
	return claveRE.ReplaceAllString(texto, "${1}"+ansiDim+"${2}="+ansiReset)
}

// colorearResumen colorea la tabla y las secciones del resumen final.
func colorearResumen(resumen string, color bool) string {
	if !color {
		return resumen
	}
	lineas := strings.Split(resumen, "\n")
	seccion := ""
	for i, l := range lineas {
		switch {
		case l == "Resumen de la corrida" || l == "Resultados" || strings.HasPrefix(l, "Avisos"):
			seccion = l
			lineas[i] = pintar(true, ansiBoldMag, l)
		case strings.HasPrefix(l, "  Módulo"):
			lineas[i] = pintar(true, ansiBold, l)
		case strings.HasPrefix(l, "  Tiempo total"):
			lineas[i] = pintar(true, ansiCyan, l)
		case l == "Sin avisos.":
			lineas[i] = pintar(true, ansiGreen, l)
		case seccion == "Resumen de la corrida":
			if m := estadoRE.FindStringSubmatchIndex(l); m != nil {
				estado := l[m[4]:m[5]]
				codigo := map[string]string{"OK": ansiBoldGrn, "ERROR": ansiBoldRed, "omitido": ansiYellow}[estado]
				lineas[i] = l[:m[2]] + pintar(true, ansiBold, strings.TrimRight(l[m[2]:m[3]], " ")) + strings.Repeat(" ", len(l[m[2]:m[3]])-len(strings.TrimRight(l[m[2]:m[3]], " "))) +
					pintar(true, codigo, estado) + l[m[4]+len(estado):]
			}
		case seccion == "Resultados":
			if m := resultaRE.FindStringSubmatch(l); m != nil {
				texto := m[4]
				if strings.HasPrefix(texto, "ERROR") {
					texto = pintar(true, ansiRed, texto)
				}
				lineas[i] = m[1] + pintar(true, ansiBold, m[2]) + m[3] + texto
			}
		case strings.HasPrefix(l, "    · "):
			lineas[i] = pintar(true, ansiYellow, l)
		case strings.HasPrefix(l, "    …"):
			lineas[i] = pintar(true, ansiDim, l)
		}
	}
	return strings.Join(lineas, "\n")
}
