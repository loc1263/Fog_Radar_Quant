// Package yahoo es el cliente común de Yahoo Finance para los 4 módulos construido sobre github.com/loc1263/gohoo.
//
// Reglas globales del proceso, compartidas por todas las goroutines:
//  1. Espaciado mínimo (+ jitter) entre los INICIOS de dos requests. Medir inicios y no
//     términos permite tener varios requests en vuelo sin subir la tasa (Fase 0, Spike A2).
//  2. Reintento con backoff exponencial + jitter ante HTTP 429 (respeta Retry-After).
//  3. Contadores (requests, 429, reintentos, espera) para el resumen de cada corrida.
//
// Las tres se aplican dentro de gohoo, por cada request HTTP (reintentos y bootstrap incluidos):
// el espaciado como gohoo.Limiter, los contadores con el hook de requests y el reintento ante
// 429 con RetryPolicy.RateLimit.
//
// Los módulos reciben una Source (interfaz) y nunca el cliente de gohoo: así los tests no usan red.
package yahoo

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loc1263/gohoo"

	"fogradar/internal/core"
)

// Policy son las reglas de tráfico.
type Policy struct {
	MinInterval time.Duration
	Jitter      time.Duration
	// Fast activa el espaciado adaptivo si es menor que MinInterval: los requests parten espaciados Fast y, ante un
	// 429, el intervalo se ensancha (nunca menos que MinInterval) y vuelve a bajar poco a poco. 0 = espaciado fijo.
	Fast        time.Duration
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
		MinInterval: sec(s.YahooMinIntervaloS), Jitter: sec(s.YahooJitterS), Fast: fastFromSettings(s),
		Retries429: s.Yahoo429Reintentos, BackoffBase: sec(s.YahooBackoffBaseS), BackoffMax: sec(s.YahooBackoffMaxS),
		NetRetries: s.YahooRedReintentos, Concurrency: max(1, s.YahooConcurrencia),
	}
}

// fastFromSettings es el intervalo inicial del modo adaptivo (0 si está apagado).
func fastFromSettings(s *core.Settings) time.Duration {
	if !s.YahooAdaptativo {
		return 0
	}
	return time.Duration(s.YahooIntervaloRapidoS * float64(time.Second))
}

// Stats son los contadores de una corrida.
type Stats struct {
	Requests    int64
	RateLimited int64
	Retries     int64
	Wait        time.Duration
	// Interval es el espaciado vigente entre requests en el modo adaptivo (0 = espaciado fijo) y Widened cuántas
	// veces se ensanchó durante la corrida.
	Interval time.Duration
	Widened  int64
}

// Summary es el texto que va al registro de jobs.
func (s Stats) Summary() string {
	out := fmt.Sprintf("%d requests a Yahoo, %d respuestas 429, %d reintentos, %.0f s de espera",
		s.Requests, s.RateLimited, s.Retries, s.Wait.Seconds())
	if s.Interval > 0 {
		out += fmt.Sprintf(", espaciado adaptivo %.2f s", s.Interval.Seconds())
		if s.Widened > 0 {
			out += fmt.Sprintf(" (ensanchado %d veces)", s.Widened)
		}
	}
	return out
}

type counters struct {
	requests, rateLimited, retries, waitNS, widened atomic.Int64
}

func (c *counters) snapshot() Stats {
	return Stats{Requests: c.requests.Load(), RateLimited: c.rateLimited.Load(), Retries: c.retries.Load(),
		Wait: time.Duration(c.waitNS.Load()), Widened: c.widened.Load()}
}

func (c *counters) reset() {
	c.requests.Store(0)
	c.rateLimited.Store(0)
	c.retries.Store(0)
	c.waitNS.Store(0)
	c.widened.Store(0)
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

// SetMin cambia el intervalo mínimo; vale para los turnos que se reserven desde ahora.
func (s *Spacer) SetMin(d time.Duration) {
	s.mu.Lock()
	s.min = max(0, d)
	s.mu.Unlock()
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

// Traffic tiene las reglas de tráfico del proceso y sus contadores; Client las instala en gohoo.
type Traffic struct {
	P     Policy
	Space *Spacer
	st    counters
	ad    *adaptive // nil = espaciado fijo
	// rlJitter es el jitter del backoff ante 429: U(0, rlJitter) (1 s; 0 en los tests).
	rlJitter time.Duration
	log      func(format string, args ...any)
}

// NewTraffic crea el controlador de tráfico.
func NewTraffic(p Policy) *Traffic {
	t := &Traffic{P: p, Space: NewSpacer(p.MinInterval, p.Jitter), rlJitter: time.Second,
		log: func(string, ...any) {}}
	if p.Fast > 0 && p.Fast < p.MinInterval {
		t.Space.SetMin(p.Fast)
		t.ad = newAdaptive(p.Fast, p.MinInterval, t.Space.SetMin)
		t.ad.onWiden = func(from, to time.Duration) {
			t.st.widened.Add(1)
			t.log("Yahoo respondió 429: el espaciado entre requests pasa de %.2f s a %.2f s", from.Seconds(), to.Seconds())
		}
	}
	return t
}

// SetLogger instala un logger de advertencias (formato printf).
func (t *Traffic) SetLogger(f func(format string, args ...any)) { t.log = f }

// Stats copia los contadores.
func (t *Traffic) Stats() Stats {
	st := t.st.snapshot()
	if t.ad != nil {
		st.Interval = t.ad.Interval()
	}
	return st
}

// ResetStats pone los contadores en cero (inicio de una corrida).
func (t *Traffic) ResetStats() { t.st.reset() }

// limiter es el espaciado global como gohoo.Limiter (cuenta la espera).
func (t *Traffic) limiter() gohoo.Limiter {
	return gohoo.LimiterFunc(func(ctx context.Context) error {
		waited, err := t.Space.Wait(ctx)
		t.st.waitNS.Add(int64(waited))
		return err
	})
}

// onRequest cuenta cada request HTTP y cada respuesta 429.
func (t *Traffic) onRequest(ev gohoo.RequestEvent) {
	t.st.requests.Add(1)
	limited := ev.StatusCode == http.StatusTooManyRequests || gohoo.IsRateLimit(ev.Err)
	if limited {
		t.st.rateLimited.Add(1)
	}
	if t.ad != nil {
		t.ad.observe(limited, ev.Err == nil)
	}
}

// retryPolicy traduce Policy a gohoo: reintentos de red cortos y, ante 429, backoff
// min(base·2^n, max) + U(0, 1 s) respetando Retry-After.
func (t *Traffic) retryPolicy() gohoo.RetryPolicy {
	return gohoo.RetryPolicy{
		MaxAttempts:     t.P.NetRetries + 1,
		BaseDelay:       200 * time.Millisecond,
		MaxDelay:        5 * time.Second,
		Jitter:          0.3,
		HonorRetryAfter: true,
		RateLimit: &gohoo.RateLimitPolicy{
			MaxRetries: t.P.Retries429, BaseDelay: t.P.BackoffBase, MaxDelay: t.P.BackoffMax,
			Jitter: t.rlJitter, HonorRetryAfter: true,
		},
		OnRetry: func(_ int, err error, delay time.Duration) {
			if !gohoo.IsRateLimit(err) {
				return
			}
			t.log("Yahoo respondió 429 (%s); reintento en %.0f s", endpointOf(err), delay.Seconds())
			t.st.retries.Add(1)
			t.st.waitNS.Add(int64(delay))
		},
	}
}

// endpointOf es la ruta del request que falló (sin query: nunca se registra el crumb).
func endpointOf(err error) string {
	var se *gohoo.StatusError
	if !errors.As(err, &se) {
		return "rate limit en la respuesta"
	}
	s := se.URL
	if i := strings.IndexByte(s, '?'); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "/v"); i >= 0 {
		s = s[i:]
	}
	if len(s) > 80 {
		s = s[len(s)-80:]
	}
	return s
}
