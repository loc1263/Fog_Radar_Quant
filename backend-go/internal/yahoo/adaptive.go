package yahoo

import (
	"sync"
	"time"
)

// Espaciado adaptivo (AIMD): el intervalo entre inicios de request parte en `fast` y se ensancha a lo seguro ante un
// 429; después baja de a poco mientras Yahoo no vuelva a limitar. Así se aprovecha el margen cuando Yahoo no
// bloquea, y un bloqueo cuesta solo unos segundos en vez de una corrida entera.
const (
	// adaptiveRecoverAfter: requests seguidos sin 429 para acercar un paso el intervalo a `fast`.
	adaptiveRecoverAfter = 50
	// adaptiveDecay es el factor de cada paso de recuperación.
	adaptiveDecay = 0.8
	// adaptiveCooldown: varios requests en vuelo reciben 429 a la vez; solo el primero ensancha el intervalo.
	adaptiveCooldown = 3 * time.Second
	// adaptiveCeil es el intervalo máximo al que puede llegar.
	adaptiveCeil = 2 * time.Second
)

type adaptive struct {
	fast, safe, ceil time.Duration
	set              func(time.Duration) // aplica el intervalo al Spacer
	now              func() time.Time
	onWiden          func(from, to time.Duration)

	mu        sync.Mutex
	cur       time.Duration
	okRun     int
	lastWiden time.Time
}

// newAdaptive crea el controlador: arranca en `fast`; tras un 429 el intervalo nunca baja de `safe`.
func newAdaptive(fast, safe time.Duration, set func(time.Duration)) *adaptive {
	return &adaptive{fast: fast, safe: safe, ceil: max(adaptiveCeil, safe), set: set, now: time.Now, cur: fast}
}

// Interval es el intervalo vigente.
func (a *adaptive) Interval() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cur
}

// observe registra el resultado de un request: limited = respondió 429; ok = terminó sin error.
// Un error de red que no es 429 no cuenta ni a favor ni en contra.
func (a *adaptive) observe(limited, ok bool) {
	a.mu.Lock()
	var from, to time.Duration
	widened := false
	switch {
	case limited:
		a.okRun = 0
		if a.lastWiden.IsZero() || a.now().Sub(a.lastWiden) >= adaptiveCooldown {
			next := min(a.ceil, max(a.cur*2, a.safe))
			if next > a.cur {
				from, to, widened = a.cur, next, true
				a.cur, a.lastWiden = next, a.now()
				a.set(next)
			}
		}
	case ok:
		a.okRun++
		if a.cur > a.fast && a.okRun >= adaptiveRecoverAfter {
			a.okRun = 0
			a.cur = max(a.fast, time.Duration(float64(a.cur)*adaptiveDecay))
			a.set(a.cur)
		}
	}
	cb := a.onWiden
	a.mu.Unlock()
	if widened && cb != nil {
		cb(from, to)
	}
}
