// Package yahoo es el cliente común de Yahoo Finance para los 4 módulos (equivale a
// fog_radar/core/yahoo.py), construido sobre github.com/loc1263/gohoo.
//
// Reglas globales del proceso, compartidas por todas las goroutines:
//  1. Espaciado mínimo (+ jitter) entre los INICIOS de dos requests. Medir inicios y no
//     términos permite tener varios requests en vuelo sin subir la tasa (Fase 0, Spike A2).
//  2. Reintento con backoff exponencial + jitter ante HTTP 429 (respeta Retry-After).
//  3. Contadores (requests, 429, reintentos, espera) para el resumen de cada corrida.
//
// Los módulos reciben una Source (interfaz) y nunca el cliente de gohoo: así los tests no usan red.
package yahoo

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loc1263/gohoo"

	"fogradar/internal/core"
)

// Policy son las reglas de tráfico (mismas variables YAHOO_* que Python).
type Policy struct {
	MinInterval time.Duration
	Jitter      time.Duration
	Retries429  int
	BackoffBase time.Duration
	BackoffMax  time.Duration
	NetRetries  int
	Concurrency int
}

// PolicyFromSettings toma la política de la configuración.
func PolicyFromSettings(s *core.Settings) Policy {
	sec := func(f float64) time.Duration { return time.Duration(f * float64(time.Second)) }
	return Policy{
		MinInterval: sec(s.YahooMinIntervaloS), Jitter: sec(s.YahooJitterS),
		Retries429: s.Yahoo429Reintentos, BackoffBase: sec(s.YahooBackoffBaseS), BackoffMax: sec(s.YahooBackoffMaxS),
		NetRetries: s.YahooRedReintentos, Concurrency: max(1, s.YahooConcurrencia),
	}
}

// Stats son los contadores de una corrida.
type Stats struct {
	Requests    int64
	RateLimited int64
	Retries     int64
	Wait        time.Duration
}

// Summary es el texto que va al registro de jobs (mismo formato que Python).
func (s Stats) Summary() string {
	return fmt.Sprintf("%d requests a Yahoo, %d respuestas 429, %d reintentos, %.0f s de espera",
		s.Requests, s.RateLimited, s.Retries, s.Wait.Seconds())
}

type counters struct {
	requests, rateLimited, retries, waitNS atomic.Int64
}

func (c *counters) snapshot() Stats {
	return Stats{c.requests.Load(), c.rateLimited.Load(), c.retries.Load(), time.Duration(c.waitNS.Load())}
}

func (c *counters) reset() {
	c.requests.Store(0)
	c.rateLimited.Store(0)
	c.retries.Store(0)
	c.waitNS.Store(0)
}

// Spacer garantiza un intervalo mínimo (con jitter) entre el inicio de dos requests.
// Reserva el siguiente turno bajo un lock y duerme fuera de él: varias goroutines quedan
// en fila sin bloquearse entre sí.
type Spacer struct {
	min, jitter time.Duration
	now         func() time.Time
	sleep       func(context.Context, time.Duration) error
	rnd         func() float64
	mu          sync.Mutex
	next        time.Time
}

// NewSpacer crea un espaciador con reloj real.
func NewSpacer(min, jitter time.Duration) *Spacer {
	return &Spacer{min: max(0, min), jitter: max(0, jitter), now: time.Now, sleep: sleepCtx, rnd: rand.Float64}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Wait espera el turno y devuelve cuánto esperó.
func (s *Spacer) Wait(ctx context.Context) (time.Duration, error) {
	s.mu.Lock()
	now := s.now()
	start := now
	if s.next.After(now) {
		start = s.next
	}
	interval := s.min
	if s.jitter > 0 {
		interval += time.Duration(s.rnd() * float64(s.jitter))
	}
	s.next = start.Add(interval)
	s.mu.Unlock()
	wait := start.Sub(now)
	if wait <= 0 {
		return 0, ctx.Err()
	}
	return wait, s.sleep(ctx, wait)
}

// Backoff es min(base·2^intento, max) + U(0, 1) s (igual que backoff_s de Python).
func Backoff(attempt int, base, maxD time.Duration, rnd func() float64) time.Duration {
	if rnd == nil {
		rnd = rand.Float64
	}
	d := base << min(attempt, 30)
	if d <= 0 || d > maxD {
		d = maxD
	}
	return d + time.Duration(rnd()*float64(time.Second))
}

type interactiveKey struct{}

// Interactive marca el contexto de una solicitud HTTP: un 429 de Yahoo se devuelve de inmediato
// (sin dormir decenas de segundos con una conexión del servidor tomada). Equivale a
// yahoo.interactivo() de Python.
func Interactive(ctx context.Context) context.Context {
	return context.WithValue(ctx, interactiveKey{}, true)
}

func isInteractive(ctx context.Context) bool { v, _ := ctx.Value(interactiveKey{}).(bool); return v }

// Traffic aplica espaciado + reintento ante 429 alrededor de una función que hace el request.
type Traffic struct {
	P     Policy
	Space *Spacer
	st    counters
	sleep func(context.Context, time.Duration) error
	rnd   func() float64
	log   func(format string, args ...any)
}

// NewTraffic crea el controlador de tráfico.
func NewTraffic(p Policy) *Traffic {
	return &Traffic{P: p, Space: NewSpacer(p.MinInterval, p.Jitter), sleep: sleepCtx, rnd: rand.Float64,
		log: func(string, ...any) {}}
}

// SetLogger instala un logger de advertencias (formato printf).
func (t *Traffic) SetLogger(f func(format string, args ...any)) { t.log = f }

// Stats copia los contadores.
func (t *Traffic) Stats() Stats { return t.st.snapshot() }

// ResetStats pone los contadores en cero (inicio de una corrida).
func (t *Traffic) ResetStats() { t.st.reset() }

// Do ejecuta fn respetando el espaciado y reintentando los 429.
func (t *Traffic) Do(ctx context.Context, what string, fn func() error) error {
	retries := t.P.Retries429
	if isInteractive(ctx) {
		retries = 0
	}
	var err error
	for attempt := 0; attempt <= retries; attempt++ {
		waited, werr := t.Space.Wait(ctx)
		if werr != nil {
			return werr
		}
		err = fn()
		t.st.requests.Add(1)
		t.st.waitNS.Add(int64(waited))
		if !gohoo.IsRateLimit(err) {
			return err
		}
		t.st.rateLimited.Add(1)
		if attempt == retries {
			break
		}
		pause := Backoff(attempt, t.P.BackoffBase, t.P.BackoffMax, t.rnd)
		var se *gohoo.StatusError
		if errors.As(err, &se) && se.RetryAfter > pause {
			pause = se.RetryAfter
		}
		t.log("Yahoo respondió 429 (%s); reintento %d/%d en %.0f s", shorten(what), attempt+1, retries, pause.Seconds())
		t.st.retries.Add(1)
		t.st.waitNS.Add(int64(pause))
		if serr := t.sleep(ctx, pause); serr != nil {
			return serr
		}
	}
	return err
}

func shorten(s string) string {
	if i := strings.IndexByte(s, '?'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 80 {
		s = s[len(s)-80:]
	}
	return s
}
