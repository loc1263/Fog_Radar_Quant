package yahoo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/loc1263/gohoo"
)

// realClient arma un Client sobre el cliente real de gohoo con las reglas de tr, redirigido a base.
func realClient(t *testing.T, tr *Traffic, base *url.URL) *Client {
	t.Helper()
	g, err := gohoo.NewClientWithOptions(append(tr.options(), gohoo.WithHTTPClient(&http.Client{Transport: rewrite{base}}))...)
	if err != nil {
		t.Fatal(err)
	}
	return newClientWith(tr, g)
}

const chartOK = `{"chart":{"result":[{"meta":{"symbol":"X","exchangeTimezoneName":"UTC"},"timestamp":[1767225600],
"indicators":{"quote":[{"open":[1],"high":[1],"low":[1],"close":[1],"volume":[1]}],"adjclose":[{"adjclose":[1]}]}}],"error":null}}`

// chartServer responde 429 las primeras n429 veces (n429 < 0: siempre) y después un chart válido.
func chartServer(t *testing.T, n429 int) (*url.URL, *atomic.Int32) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n := int(calls.Add(1)); n429 < 0 || n <= n429 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = fmt.Fprint(w, chartOK)
	}))
	t.Cleanup(srv.Close)
	base, _ := url.Parse(srv.URL)
	return base, &calls
}

func TestTrafficReintenta429ConBackoffYCuenta(t *testing.T) {
	tr, _ := newRelojTraffic(Policy{Retries429: 3, BackoffBase: time.Millisecond, BackoffMax: 2 * time.Millisecond})
	var mu sync.Mutex
	var logs []string
	tr.SetLogger(func(f string, a ...any) { mu.Lock(); logs = append(logs, fmt.Sprintf(f, a...)); mu.Unlock() })
	base, calls := chartServer(t, 2)

	res, err := realClient(t, tr, base).Chart(context.Background(), "X", gohoo.ChartParams{Range: "1y"})
	if err != nil || len(res.Bars) != 1 || calls.Load() != 3 {
		t.Fatalf("res=%v err=%v llamadas=%d", res, err, calls.Load())
	}
	st := tr.Stats()
	// esperas del backoff: 1 ms y 2 ms (jitter 0 en el test)
	if st.Requests != 3 || st.RateLimited != 2 || st.Retries != 2 || st.Wait != 3*time.Millisecond {
		t.Fatalf("%+v", st)
	}
	if st.Summary() != "3 requests a Yahoo, 2 respuestas 429, 2 reintentos, 0 s de espera" {
		t.Fatal(st.Summary())
	}
	if len(logs) != 2 || !strings.Contains(logs[0], "/v8/finance/chart/X") || strings.Contains(logs[0], "crumb") {
		t.Fatalf("logs: %v", logs)
	}
	tr.ResetStats()
	if tr.Stats().Requests != 0 {
		t.Fatal("reset")
	}
}

func TestTrafficDevuelve429SiSeAgotanLosReintentos(t *testing.T) {
	tr, _ := newRelojTraffic(Policy{Retries429: 1, BackoffBase: time.Millisecond, BackoffMax: time.Millisecond})
	base, calls := chartServer(t, -1)
	_, err := realClient(t, tr, base).Chart(context.Background(), "X", gohoo.ChartParams{Range: "1y"})
	if !gohoo.IsRateLimit(err) || calls.Load() != 2 || tr.Stats().RateLimited != 2 {
		t.Fatal(err, calls.Load(), tr.Stats())
	}
}

func TestTrafficInteractivoNoReintenta(t *testing.T) {
	tr, _ := newRelojTraffic(Policy{Retries429: 3, BackoffBase: time.Minute, BackoffMax: time.Minute})
	base, calls := chartServer(t, -1)
	_, err := realClient(t, tr, base).Chart(Interactive(context.Background()), "X", gohoo.ChartParams{Range: "1y"})
	if !gohoo.IsRateLimit(err) || calls.Load() != 1 {
		t.Fatal(err, calls.Load())
	}
}

func TestTrafficOtrosErroresNoSeReintentan(t *testing.T) {
	tr, _ := newRelojTraffic(Policy{Retries429: 3})
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	base, _ := url.Parse(srv.URL)
	_, err := realClient(t, tr, base).Chart(context.Background(), "NOPE", gohoo.ChartParams{Range: "1y"})
	if !errors.Is(err, gohoo.ErrNotFound) || calls.Load() != 1 {
		t.Fatal(err, calls.Load())
	}
}

func TestTrafficEspaciaCadaRequestIncluidosReintentos(t *testing.T) {
	tr, r := newRelojTraffic(Policy{MinInterval: time.Second, Retries429: 2, BackoffBase: time.Millisecond, BackoffMax: time.Millisecond})
	base, _ := chartServer(t, 2)
	if _, err := realClient(t, tr, base).Chart(context.Background(), "X", gohoo.ChartParams{Range: "1y"}); err != nil {
		t.Fatal(err)
	}
	// El reloj falso no avanza durante el backoff real (1 ms): el 2.º y 3.er request esperan su turno de 1 s.
	if len(r.dormidos) != 2 || r.dormidos[0] != time.Second || r.dormidos[1] != time.Second {
		t.Fatalf("esperas del espaciado: %v", r.dormidos)
	}
	if st := tr.Stats(); st.Wait != 2*time.Second+2*time.Millisecond {
		t.Fatalf("espera total: %v", st.Wait)
	}
}

func TestTrafficPoliticaDeReintentos(t *testing.T) {
	tr := NewTraffic(Policy{Retries429: 3, BackoffBase: 5 * time.Second, BackoffMax: time.Minute, NetRetries: 2})
	p := tr.retryPolicy(false)
	if p.MaxAttempts != 3 || !p.HonorRetryAfter || p.RateLimit == nil {
		t.Fatalf("%+v", p)
	}
	if rl := *p.RateLimit; rl.MaxRetries != 3 || rl.BaseDelay != 5*time.Second || rl.MaxDelay != time.Minute || rl.Jitter != time.Second || !rl.HonorRetryAfter {
		t.Fatalf("%+v", rl)
	}
	if tr.retryPolicy(true).RateLimit.MaxRetries != 0 {
		t.Fatal("el modo interactivo no reintenta los 429")
	}
}
