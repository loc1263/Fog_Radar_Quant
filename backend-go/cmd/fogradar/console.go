package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// consoleHandler es la salida de `fogradar job` en la consola, pensada para leerse: una línea por hito con la hora
// («23:34:46  INFO   mensaje  clave=valor») y el avance de los recorridos largos como una barra que se actualiza
// cada 10 %, en vez de una línea por ticker (un job de ETF llegaba a miles). El log único
// data/_jobs/logs/fogradar.log no se toca: guarda todos los registros con su detalle. Con LOG_LEVEL=DEBUG la consola
// también lo muestra todo.
type consoleHandler struct {
	w       io.Writer
	level   slog.Level
	verbose bool
	st      *consoleState
	attrs   []slog.Attr
}

type consoleState struct {
	mu      sync.Mutex
	avances map[string]*avanceBarra // por etiqueta de avance
}

// avanceBarra es lo último mostrado de un recorrido (varios hilos lo reportan fuera de orden).
type avanceBarra struct {
	cur, tramo int
	terminado  time.Time // cuándo llegó al total (cero = en curso)
}

// tardanzaFinal: los registros atrasados de un recorrido recién terminado no abren uno nuevo.
const tardanzaFinal = 2 * time.Second

func newConsoleHandler(w io.Writer, lvl slog.Level) *consoleHandler {
	return &consoleHandler{w: w, level: lvl, verbose: lvl <= slog.LevelDebug, st: &consoleState{avances: map[string]*avanceBarra{}}}
}

func (h *consoleHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

func (h *consoleHandler) WithAttrs(a []slog.Attr) slog.Handler {
	c := *h
	c.attrs = append(append([]slog.Attr{}, h.attrs...), a...)
	return &c
}

func (h *consoleHandler) WithGroup(string) slog.Handler { return h }

const (
	maxValorConsola = 140 // caracteres de un valor antes de cortarlo con «…»
	anchoBarra      = 20
)

var (
	avanceAttrRE = regexp.MustCompile(`^(\d+)/(\d+)`)
	avanceMsgRE  = regexp.MustCompile(`^(.*?)\s*\((\d+)/(\d+)\)`)
)

func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := append([]slog.Attr{}, h.attrs...)
	r.Attrs(func(a slog.Attr) bool { attrs = append(attrs, a); return true })

	if !h.verbose && r.Level < slog.LevelWarn {
		if enSilencio(r.Message) {
			return nil
		}
		if etiqueta, cur, total, ok := avance(r.Message, attrs); ok {
			linea, mostrar := h.st.barra(etiqueta, cur, total, r.Time)
			if !mostrar {
				return nil
			}
			return h.escribir(r, linea, false)
		}
	}

	var b strings.Builder
	b.WriteString(r.Message)
	var cabecera string
	for _, a := range attrs {
		if a.Key == "proc" || a.Key == "pid" {
			continue
		}
		v := a.Value.Resolve().String()
		switch {
		case r.Message == "inicio" && a.Key == "modulo": // un módulo nuevo: línea en blanco y encabezado
			cabecera = v
		case r.Message == "OK" && a.Key == "mensaje": // el resultado ya sale en el resumen final
		case r.Message == "inicio" && a.Key == "opciones" && v == "map[]":
		default:
			b.WriteString("  " + a.Key + "=" + valorConsola(v))
		}
	}
	if cabecera != "" && !h.verbose {
		return h.escribir(r, ">>> "+cabecera+strings.TrimPrefix(b.String(), "inicio"), true)
	}
	return h.escribir(r, b.String(), false)
}

// escribir imprime una línea; blanco antepone una línea en blanco (separa los módulos).
func (h *consoleHandler) escribir(r slog.Record, texto string, blanco bool) error {
	tag := "INFO "
	switch {
	case r.Level >= slog.LevelError:
		tag = "ERROR"
	case r.Level >= slog.LevelWarn:
		tag = "AVISO"
	case r.Level < slog.LevelInfo:
		tag = "DEBUG"
	}
	h.st.mu.Lock()
	defer h.st.mu.Unlock()
	if blanco {
		_, _ = fmt.Fprintln(h.w)
	}
	_, err := fmt.Fprintf(h.w, "%s  %s  %s\n", r.Time.Format("15:04:05"), tag, texto)
	return err
}

// enSilencio: registros por ticker que solo valen para el log de archivo.
func enSilencio(msg string) bool {
	// «resultado» repite, al cerrar la secuencia, lo que muestra la tabla del resumen.
	return strings.HasPrefix(msg, "archivado:") || msg == "evaluado" || msg == "resultado"
}

// avance detecta un registro de avance («n=3/30», «n="701/1226 | 57.2%"» o «[Asia] [sector] (3/20) SYM: x»).
func avance(msg string, attrs []slog.Attr) (etiqueta string, cur, total int, ok bool) {
	for _, a := range attrs {
		if a.Key != "n" {
			continue
		}
		if m := avanceAttrRE.FindStringSubmatch(a.Value.Resolve().String()); m != nil {
			cur, _ = strconv.Atoi(m[1])
			total, _ = strconv.Atoi(m[2])
			return strings.TrimSuffix(msg, " inválido"), cur, total, total > 0
		}
	}
	if m := avanceMsgRE.FindStringSubmatch(msg); m != nil {
		cur, _ = strconv.Atoi(m[2])
		total, _ = strconv.Atoi(m[3])
		return m[1], cur, total, total > 0
	}
	return "", 0, 0, false
}

// barra devuelve la línea de avance y si toca mostrarla: la primera, cada décimo nuevo y la última. Con varios hilos los
// registros llegan fuera de orden: solo se muestran los que avanzan (nunca un 1/30 después de un 3/30).
func (s *consoleState) barra(etiqueta string, cur, total int, ahora time.Time) (string, bool) {
	cur = min(max(cur, 0), total)
	s.mu.Lock()
	defer s.mu.Unlock()
	tramo := cur * 10 / total
	a, visto := s.avances[etiqueta]
	if visto && !a.terminado.IsZero() { // recorrido terminado: un 1/N o un registro tardío (> tardanzaFinal) abre otro
		if cur == 1 || ahora.Sub(a.terminado) > tardanzaFinal {
			visto = false
		} else {
			return "", false
		}
	}
	if visto && (cur <= a.cur || (tramo <= a.tramo && cur != total)) {
		return "", false
	}
	a = &avanceBarra{cur: cur, tramo: tramo}
	if cur == total {
		a.terminado = ahora
	}
	s.avances[etiqueta] = a
	lleno := cur * anchoBarra / total
	return fmt.Sprintf("%-18s [%s%s] %3d%%  %d/%d", etiqueta, strings.Repeat("#", lleno), strings.Repeat("-", anchoBarra-lleno),
		cur*100/total, cur, total), true
}

// valorConsola acorta los valores muy largos y pone comillas a los que tienen espacios.
func valorConsola(v string) string {
	if utf8.RuneCountInString(v) > maxValorConsola {
		v = string([]rune(v)[:maxValorConsola]) + "…"
	}
	if v == "" || strings.ContainsAny(v, " =\"") {
		return strconv.Quote(v)
	}
	return v
}
