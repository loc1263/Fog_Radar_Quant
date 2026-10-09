package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Aviso es un WARN/ERROR emitido por un módulo durante su corrida. Ticker viene del atributo «ticker»
// del registro (vacío si el aviso no es de un ticker concreto).
type Aviso struct {
	Ticker  string
	Mensaje string
}

// avisosHandler envuelve el handler del proceso y anota los WARN/ERROR del módulo en curso.
type avisosHandler struct {
	slog.Handler
	c *avisosCollector
}

type avisosCollector struct {
	mu     sync.Mutex
	avisos []Aviso
}

func (h avisosHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn {
		var a Aviso
		var extra []string
		r.Attrs(func(at slog.Attr) bool {
			switch at.Key {
			case "ticker":
				a.Ticker = at.Value.String()
			default:
				extra = append(extra, fmt.Sprintf("%s=%v", at.Key, at.Value))
			}
			return true
		})
		a.Mensaje = r.Message
		if len(extra) > 0 {
			a.Mensaje += " (" + strings.Join(extra, ", ") + ")"
		}
		h.c.mu.Lock()
		h.c.avisos = append(h.c.avisos, a)
		h.c.mu.Unlock()
	}
	return h.Handler.Handle(ctx, r)
}

func (h avisosHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return avisosHandler{h.Handler.WithAttrs(a), h.c}
}

func (h avisosHandler) WithGroup(n string) slog.Handler {
	return avisosHandler{h.Handler.WithGroup(n), h.c}
}

// conAvisos devuelve un logger que además de escribir anota los WARN/ERROR, y una función que entrega lo
// anotado desde la última llamada (en orden, sin repetidos exactos).
func conAvisos(base *slog.Logger) (*slog.Logger, func() []Aviso) {
	c := &avisosCollector{}
	take := func() []Aviso {
		c.mu.Lock()
		defer c.mu.Unlock()
		seen := map[Aviso]bool{}
		var out []Aviso
		for _, a := range c.avisos {
			if !seen[a] {
				seen[a] = true
				out = append(out, a)
			}
		}
		c.avisos = nil
		return out
	}
	return slog.New(avisosHandler{base.Handler(), c}), take
}

// HMS formatea una duración como HH:MM:SS (las horas pueden pasar de 24).
func HMS(d time.Duration) string {
	s := int(d.Round(time.Second).Seconds())
	return fmt.Sprintf("%02d:%02d:%02d", s/3600, s%3600/60, s%60)
}

// maxAvisosPorModulo limita las líneas de avisos de un módulo en el resumen (el detalle completo queda en fogradar.log).
const maxAvisosPorModulo = 40

// formatAvisos arma la sección de avisos del resumen: por módulo, primero los de tickers y luego los generales.
func formatAvisos(res []Resultado) string {
	var b strings.Builder
	for _, r := range res {
		if len(r.Avisos) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n  %s — %d avisos:\n", r.Modulo, len(r.Avisos))
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
				b.WriteString("    " + s + "\n")
			}
			n++
		}
		for _, a := range tickers {
			linea(fmt.Sprintf("%-12s %s", a.Ticker, a.Mensaje))
		}
		for _, a := range otros {
			linea("· " + a.Mensaje)
		}
		if n > maxAvisosPorModulo {
			fmt.Fprintf(&b, "    … y %d más (ver data/_jobs/logs/fogradar.log)\n", n-maxAvisosPorModulo)
		}
	}
	return b.String()
}
