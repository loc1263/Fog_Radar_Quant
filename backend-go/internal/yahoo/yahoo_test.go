package yahoo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/loc1263/gohoo"
)

// reloj falso: sleep avanza el tiempo.
type reloj struct {
	mu       sync.Mutex
	t        time.Time
	dormidos []time.Duration
}

func (r *reloj) now() time.Time { r.mu.Lock(); defer r.mu.Unlock(); return r.t }
func (r *reloj) sleep(_ context.Context, d time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dormidos = append(r.dormidos, d)
	r.t = r.t.Add(d)
	return nil
}

func newRelojTraffic(p Policy) (*Traffic, *reloj) {
	r := &reloj{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	tr := NewTraffic(p)
	tr.Space.now, tr.Space.sleep, tr.Space.rnd = r.now, r.sleep, func() float64 { return 0 }
	tr.sleep, tr.rnd = r.sleep, func() float64 { return 0 }
	return tr, r
}

func TestSpacerRespetaIntervaloMinimo(t *testing.T) {
	r := &reloj{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	s := NewSpacer(time.Second, 0)
	s.now, s.sleep = r.now, r.sleep
	ctx := context.Background()
	if w, _ := s.Wait(ctx); w != 0 {
		t.Fatal("la primera no espera")
	}
	if w, _ := s.Wait(ctx); w != time.Second {
		t.Fatalf("la segunda espera 1 s: %v", w)
	}
	r.t = r.t.Add(5 * time.Second)
	if w, _ := s.Wait(ctx); w != 0 {
		t.Fatal("pasó tiempo de sobra")
	}
}

func TestSpacerEntreGoroutinesSerializaInicios(t *testing.T) {
	s := NewSpacer(20*time.Millisecond, 0)
	var mu sync.Mutex
	var starts []time.Time
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Wait(context.Background()); err != nil {
				t.Error(err)
			}
			mu.Lock()
			starts = append(starts, time.Now())
			mu.Unlock()
		}()
	}
	wg.Wait()
	for i := range starts { // ordenar
		for j := i + 1; j < len(starts); j++ {
			if starts[j].Before(starts[i]) {
				starts[i], starts[j] = starts[j], starts[i]
			}
		}
	}
	for i := 1; i < len(starts); i++ {
		if starts[i].Sub(starts[i-1]) < 15*time.Millisecond {
			t.Fatalf("inicios muy juntos: %v", starts[i].Sub(starts[i-1]))
		}
	}
}

func TestSpacerCancelable(t *testing.T) {
	s := NewSpacer(time.Hour, 0)
	_, _ = s.Wait(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := s.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("esperaba deadline: %v", err)
	}
}

var err429 = fmt.Errorf("agotados 1 intentos: %w", gohoo.ErrRateLimit)

func TestDoReintenta429ConBackoff(t *testing.T) {
	tr, r := newRelojTraffic(Policy{Retries429: 3, BackoffBase: 2 * time.Second, BackoffMax: 5 * time.Second})
	resp := []error{err429, err429, nil}
	i := 0
	err := tr.Do(context.Background(), "chart/X", func() error { e := resp[i]; i++; return e })
	if err != nil || i != 3 {
		t.Fatal(err, i)
	}
	// backoff: 2·2^0 y 2·2^1 (el jitter U(0,1) es 0 en el test)
	if len(r.dormidos) != 2 || r.dormidos[0] != 2*time.Second || r.dormidos[1] != 4*time.Second {
		t.Fatalf("dormidos: %v", r.dormidos)
	}
	st := tr.Stats()
	if st.Requests != 3 || st.RateLimited != 2 || st.Retries != 2 || st.Wait != 6*time.Second {
		t.Fatalf("%+v", st)
	}
	if st.Summary() != "3 requests a Yahoo, 2 respuestas 429, 2 reintentos, 6 s de espera" {
		t.Fatal(st.Summary())
	}
	tr.ResetStats()
	if tr.Stats().Requests != 0 {
		t.Fatal("reset")
	}
}

func TestDoDevuelve429SiSeAgotanLosReintentos(t *testing.T) {
	tr, r := newRelojTraffic(Policy{Retries429: 1})
	err := tr.Do(context.Background(), "x", func() error { return err429 })
	if !gohoo.IsRateLimit(err) || len(r.dormidos) != 1 {
		t.Fatal(err, r.dormidos)
	}
}

func TestDoRespetaRetryAfter(t *testing.T) {
	tr, r := newRelojTraffic(Policy{Retries429: 1, BackoffBase: time.Second, BackoffMax: time.Second})
	calls := 0
	_ = tr.Do(context.Background(), "x", func() error {
		calls++
		if calls == 1 {
			return &gohoo.StatusError{StatusCode: 429, RetryAfter: 30 * time.Second}
		}
		return nil
	})
	if len(r.dormidos) != 1 || r.dormidos[0] != 30*time.Second {
		t.Fatalf("debe esperar el Retry-After: %v", r.dormidos)
	}
}

func TestInteractivoNoReintenta(t *testing.T) {
	tr, r := newRelojTraffic(Policy{Retries429: 3, BackoffBase: time.Minute, BackoffMax: time.Minute})
	n := 0
	err := tr.Do(Interactive(context.Background()), "x", func() error { n++; return err429 })
	if !gohoo.IsRateLimit(err) || n != 1 || len(r.dormidos) != 0 {
		t.Fatal(err, n, r.dormidos)
	}
}

func TestOtrosErroresNoSeReintentan(t *testing.T) {
	tr, _ := newRelojTraffic(Policy{Retries429: 3})
	n := 0
	boom := errors.New("boom")
	if err := tr.Do(context.Background(), "x", func() error { n++; return boom }); !errors.Is(err, boom) || n != 1 {
		t.Fatal(err, n)
	}
}

func TestBackoffTieneTope(t *testing.T) {
	if d := Backoff(10, 5*time.Second, 60*time.Second, func() float64 { return 1 }); d > 61*time.Second {
		t.Fatal(d)
	}
	if d := Backoff(100, time.Second, time.Minute, func() float64 { return 0 }); d != time.Minute {
		t.Fatalf("desborde de shift: %v", d)
	}
}

// --- históricos ----------------------------------------------------------------------

type fakeBackend struct {
	calls atomic.Int64
	fn    func(n int, ticker string) (*gohoo.ChartResult, error)
}

func (f *fakeBackend) ChartRange(_ context.Context, ticker string, _ gohoo.ChartParams) (*gohoo.ChartResult, error) {
	n := int(f.calls.Add(1))
	return f.fn(n, ticker)
}
func (f *fakeBackend) QuoteSummaryContext(context.Context, string, []string) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}
func (f *fakeBackend) FundamentalsTimeseriesContext(context.Context, string, []string) (map[string]any, error) {
	return nil, nil
}

func bars(n int) []gohoo.Bar {
	loc := time.FixedZone("EST", -5*3600)
	out := make([]gohoo.Bar, n)
	for i := range out {
		c := float64(i + 1)
		out[i] = gohoo.Bar{Time: time.Date(2026, 1, 2+7*i, 0, 0, 0, 0, loc), Open: c, High: c + 1, Low: c - 1,
			Close: c, AdjClose: c / 2, Volume: 100 * c}
	}
	return out
}

func newHistClient(fn func(n int, ticker string) (*gohoo.ChartResult, error)) (*Client, *fakeBackend) {
	tr, _ := newRelojTraffic(Policy{})
	fb := &fakeBackend{fn: fn}
	return newClientWith(tr, fb), fb
}

func fastOpts() HistoryOptions {
	o := DefaultHistoryOptions()
	o.BackoffBase, o.BackoffMax = time.Millisecond, time.Millisecond
	o.MaxRetries = 3
	return o
}

func TestHistoryReintentaYCachea(t *testing.T) {
	dir := t.TempDir()
	c, fb := newHistClient(func(n int, _ string) (*gohoo.ChartResult, error) {
		if n == 1 {
			return nil, errors.New("Too Many Requests")
		}
		return &gohoo.ChartResult{Bars: bars(3), Dividends: map[string]float64{"2026-01-09": 0.5}}, nil
	})
	ctx := context.Background()
	got, err := c.History(ctx, "7203.T", dir, fastOpts(), false)
	if err != nil || len(got) != 3 || fb.calls.Load() != 2 {
		t.Fatal(err, len(got), fb.calls.Load())
	}
	if _, err := os.Stat(filepath.Join(dir, "7203.T.csv")); err != nil {
		t.Fatal("falta el CSV")
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(m) != 0 {
		t.Fatal("quedó un .tmp")
	}
	// auto_adjust: Close = AdjClose y OHL escalados con AdjClose/Close (=0.5)
	if got[1].Close != 1 || got[1].Open != 1 || got[1].High != 1.5 {
		t.Fatalf("ajuste: %+v", got[1])
	}
	// Segunda vez: sale de la caché.
	again, err := c.History(ctx, "7203.T", dir, fastOpts(), false)
	if err != nil || len(again) != 3 || fb.calls.Load() != 2 {
		t.Fatal(err, fb.calls.Load())
	}
	if !again[0].Time.Equal(got[0].Time) || again[2].Close != got[2].Close || again[2].Volume != got[2].Volume {
		t.Fatalf("la caché cambió los datos: %+v vs %+v", again[2], got[2])
	}
	if _, err := c.History(ctx, "7203.T", dir, fastOpts(), true); err != nil || fb.calls.Load() != 3 {
		t.Fatal("force_refresh debe volver a pedir", fb.calls.Load())
	}
}

func TestHistoryNoReintentaSimboloInexistente(t *testing.T) {
	c, fb := newHistClient(func(int, string) (*gohoo.ChartResult, error) { return nil, gohoo.ErrNotFound })
	_, err := c.History(context.Background(), "NOPE", t.TempDir(), fastOpts(), false)
	if !errors.Is(err, gohoo.ErrNotFound) || fb.calls.Load() != 1 {
		t.Fatal(err, fb.calls.Load())
	}
}

func TestHistoryFalloDefinitivoYVacio(t *testing.T) {
	c, fb := newHistClient(func(int, string) (*gohoo.ChartResult, error) { return &gohoo.ChartResult{}, nil })
	if _, err := c.History(context.Background(), "X", t.TempDir(), fastOpts(), false); err == nil || fb.calls.Load() != 3 {
		t.Fatal(err, fb.calls.Load())
	}
}

func TestCachePathYFrescura(t *testing.T) {
	if p := CachePath("d", "^GSPC"); p != filepath.Join("d", "_GSPC.csv") {
		t.Fatal(p)
	}
	if p := CachePath("d", "USDJPY=X"); p != filepath.Join("d", "USDJPY_X.csv") {
		t.Fatal(p)
	}
	f := filepath.Join(t.TempDir(), "a.csv")
	if CacheFresh(f, time.Hour) {
		t.Fatal("no existe")
	}
	_ = os.WriteFile(f, []byte("x"), 0o644)
	if !CacheFresh(f, time.Hour) || CacheFresh(f, 0) {
		t.Fatal("frescura")
	}
	_ = os.Chtimes(f, time.Now().Add(-48*time.Hour), time.Now().Add(-48*time.Hour))
	if CacheFresh(f, 20*time.Hour) {
		t.Fatal("vieja")
	}
}

func TestReadBarsCSVFormatoEstandar(t *testing.T) {
	f := filepath.Join(t.TempDir(), "p.csv")
	csv := "Date,Open,High,Low,Close,Volume,Dividends,Stock Splits\n" +
		"2021-01-04 00:00:00-05:00,1.5,2,1,1.75,1000,0.0,0.0\n" +
		"2021-01-11 00:00:00-05:00,,,,1.8,,0.0,0.0\n"
	_ = os.WriteFile(f, []byte(csv), 0o644)
	b, err := ReadBarsCSV(f)
	if err != nil || len(b) != 2 {
		t.Fatal(err, b)
	}
	if b[0].Close != 1.75 || b[0].Time.Format("2006-01-02") != "2021-01-04" || !math.IsNaN(b[1].Open) {
		t.Fatalf("%+v", b)
	}
	_ = os.WriteFile(f, []byte("Date,Open\n2021-01-04,1\n"), 0o644)
	if _, err := ReadBarsCSV(f); err == nil {
		t.Fatal("sin Close debe fallar")
	}
}

func TestDownloadManyConcurrenteConservaOrdenYFallos(t *testing.T) {
	c, _ := newHistClient(func(_ int, tk string) (*gohoo.ChartResult, error) {
		if tk == "BAD1" || tk == "BAD2" {
			return nil, gohoo.ErrNotFound
		}
		return &gohoo.ChartResult{Bars: bars(2)}, nil
	})
	tickers := []string{"A", "BAD1", "B", "C", "BAD2", "D"}
	var seen atomic.Int64
	out, failed := c.DownloadMany(context.Background(), tickers, t.TempDir(), fastOpts(), false, 3,
		func(i, n int, tk string) { seen.Add(1) })
	if len(out) != 4 || len(failed) != 2 || failed[0] != "BAD1" || failed[1] != "BAD2" || seen.Load() != 6 {
		t.Fatal(len(out), failed, seen.Load())
	}
}

func TestClientDelegaEnElBackend(t *testing.T) {
	c, _ := newHistClient(nil)
	m, err := c.QuoteSummary(context.Background(), "AAPL", []string{"price"})
	if err != nil || m["ok"] != true {
		t.Fatal(m, err)
	}
	if c.T.Stats().Requests != 1 {
		t.Fatal("debe contar el request")
	}
}
