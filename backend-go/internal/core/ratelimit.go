package core

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var unidades = map[string]time.Duration{
	"second": time.Second, "minute": time.Minute, "hour": time.Hour, "day": 24 * time.Hour,
}

// ParseLimit interpreta "300/minute" (también "5/second", "10/hour", "1/day").
func ParseLimit(s string) (int, time.Duration, error) {
	n, unit, ok := strings.Cut(strings.TrimSpace(s), "/")
	if !ok {
		return 0, 0, fmt.Errorf("límite inválido %q (formato N/unidad)", s)
	}
	count, err := strconv.Atoi(strings.TrimSpace(n))
	if err != nil || count < 1 {
		return 0, 0, fmt.Errorf("límite inválido %q", s)
	}
	d, ok := unidades[strings.TrimSuffix(strings.ToLower(strings.TrimSpace(unit)), "s")]
	if !ok {
		return 0, 0, fmt.Errorf("unidad inválida en %q (second|minute|hour|day)", s)
	}
	return count, d, nil
}

// Limiter es una ventana deslizante en memoria por (scope, clave), como
// MovingWindowRateLimiter de `limits`. Por proceso: suficiente como red de seguridad.
type Limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
	now  func() time.Time
}

// NewLimiter crea un limitador vacío.
func NewLimiter() *Limiter { return &Limiter{hits: map[string][]time.Time{}, now: time.Now} }

// Hit registra una solicitud y dice si cabe en el límite.
func (l *Limiter) Hit(count int, window time.Duration, scope, key string) bool {
	k := scope + "\x00" + key
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	h := l.hits[k]
	cut := 0
	for cut < len(h) && now.Sub(h[cut]) >= window {
		cut++
	}
	h = h[cut:]
	if len(h) >= count {
		l.hits[k] = h
		return false
	}
	l.hits[k] = append(h, now)
	return true
}

// ClientKey es la IP del usuario final: el BFF la reenvía en X-Forwarded-For
// (confiable: el backend solo acepta requests del servidor frontend).
func ClientKey(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		first, _, _ := strings.Cut(fwd, ",")
		return strings.TrimSpace(first)
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = strings.Trim(host[:i], "[]")
	}
	if host == "" {
		return "desconocido"
	}
	return host
}

// Check aplica un límite "N/unidad" a la solicitud; devuelve el error 429 si se excede.
func (l *Limiter) Check(s *Settings, limit, scope string, r *http.Request) *APIError {
	if s.Env == "test" && !s.RateLimitInTests {
		return nil
	}
	count, window, err := ParseLimit(limit)
	if err != nil {
		return Err(http.StatusInternalServerError, "internal_error", err.Error())
	}
	if !l.Hit(count, window, scope, ClientKey(r)) {
		return Err(http.StatusTooManyRequests, "rate_limited",
			fmt.Sprintf("Demasiadas solicitudes (límite %s). Intenta más tarde.", limit))
	}
	return nil
}

// Gate limita las ejecuciones simultáneas de un endpoint costoso (OWASP API4).
// Si está lleno no espera: el llamador responde 503 "busy".
type Gate struct{ sem chan struct{} }

// NewGate permite como máximo n ejecuciones simultáneas.
func NewGate(n int) *Gate { return &Gate{sem: make(chan struct{}, n)} }

// Acquire intenta tomar un cupo; devuelve el release, o un APIError 503 si está lleno.
func (g *Gate) Acquire() (release func(), err *APIError) {
	select {
	case g.sem <- struct{}{}:
		return func() { <-g.sem }, nil
	default:
		return nil, Err(http.StatusServiceUnavailable, "busy",
			"El servidor está procesando otras solicitudes de este tipo. Reintenta.")
	}
}
