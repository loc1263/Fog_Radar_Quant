package yahoo

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/loc1263/gohoo"
)

// nuevoAdaptivo arma un controlador 0,2 s → seguro 0,35 s con reloj propio y registro de los intervalos aplicados.
func nuevoAdaptivo() (*adaptive, *time.Time, *[]time.Duration) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var applied []time.Duration
	a := newAdaptive(200*time.Millisecond, 350*time.Millisecond, func(d time.Duration) { applied = append(applied, d) })
	a.now = func() time.Time { return now }
	return a, &now, &applied
}

func TestAdaptivoEnsanchaAnteUn429SinBajarDelSeguro(t *testing.T) {
	a, _, applied := nuevoAdaptivo()
	if a.Interval() != 200*time.Millisecond {
		t.Fatalf("parte en %v, quería 200ms", a.Interval())
	}
	a.observe(true, false)
	if got := a.Interval(); got != 400*time.Millisecond { // max(2×0,2; 0,35)
		t.Errorf("tras el 1.er 429: %v, quería 400ms", got)
	}
	if len(*applied) != 1 {
		t.Errorf("debía aplicarse al Spacer una vez: %v", *applied)
	}
}

func TestAdaptivoIgnora429SimultaneosYLuegoSigueEnsanchando(t *testing.T) {
	a, now, _ := nuevoAdaptivo()
	a.observe(true, false) // 0,4 s
	a.observe(true, false) // los otros requests en vuelo también reciben 429: no se duplica de nuevo
	a.observe(true, false)
	if got := a.Interval(); got != 400*time.Millisecond {
		t.Fatalf("429 simultáneos: %v, quería 400ms", got)
	}
	*now = now.Add(adaptiveCooldown)
	a.observe(true, false)
	if got := a.Interval(); got != 800*time.Millisecond {
		t.Errorf("tras el enfriamiento: %v, quería 800ms", got)
	}
	for range 5 { // nunca pasa del techo
		*now = now.Add(adaptiveCooldown)
		a.observe(true, false)
	}
	if got := a.Interval(); got != adaptiveCeil {
		t.Errorf("techo: %v, quería %v", got, adaptiveCeil)
	}
}

func TestAdaptivoRecuperaDeAPocoSinBajarDelRapido(t *testing.T) {
	a, _, _ := nuevoAdaptivo()
	a.observe(true, false) // 0,4 s
	for range adaptiveRecoverAfter - 1 {
		a.observe(false, true)
	}
	if a.Interval() != 400*time.Millisecond {
		t.Fatalf("antes de %d requests limpios no debe bajar: %v", adaptiveRecoverAfter, a.Interval())
	}
	a.observe(false, true)
	if got := a.Interval(); got != 320*time.Millisecond { // 0,4 × 0,8
		t.Errorf("1.er paso: %v, quería 320ms", got)
	}
	for range 20 * adaptiveRecoverAfter {
		a.observe(false, true)
	}
	if got := a.Interval(); got != 200*time.Millisecond {
		t.Errorf("tras mucho tráfico limpio vuelve al rápido: %v", got)
	}
}

func TestAdaptivoUn429CortaLaRachaYUnErrorDeRedNoCuenta(t *testing.T) {
	a, now, _ := nuevoAdaptivo()
	a.observe(true, false) // 0,4 s
	for range adaptiveRecoverAfter - 1 {
		a.observe(false, true)
	}
	*now = now.Add(adaptiveCooldown)
	a.observe(true, false) // otro 429 justo antes de recuperar: ensancha y reinicia la racha
	if a.Interval() != 800*time.Millisecond {
		t.Fatalf("%v", a.Interval())
	}
	for range adaptiveRecoverAfter - 1 {
		a.observe(false, false) // errores de red: ni suman a la racha ni ensanchan
	}
	a.observe(false, true)
	if a.Interval() != 800*time.Millisecond {
		t.Errorf("los errores de red no cuentan como éxito: %v", a.Interval())
	}
}

func TestTrafficAdaptivoSoloSiFastEsMenorQueElMinimo(t *testing.T) {
	for name, p := range map[string]Policy{
		"sin Fast":         {MinInterval: 350 * time.Millisecond},
		"Fast igual":       {MinInterval: 350 * time.Millisecond, Fast: 350 * time.Millisecond},
		"Fast más lento":   {MinInterval: 350 * time.Millisecond, Fast: time.Second},
		"sin MinInterval":  {Fast: 200 * time.Millisecond},
		"política vacía":   {},
		"espaciado grande": {MinInterval: time.Second, Fast: 0},
	} {
		if tr := NewTraffic(p); tr.ad != nil || tr.Stats().Interval != 0 {
			t.Errorf("%s: no debe ser adaptivo", name)
		}
	}
	tr := NewTraffic(Policy{MinInterval: 350 * time.Millisecond, Fast: 200 * time.Millisecond})
	if tr.ad == nil || tr.Space.min != 200*time.Millisecond || tr.Stats().Interval != 200*time.Millisecond {
		t.Fatalf("debe partir en Fast: min=%v stats=%+v", tr.Space.min, tr.Stats())
	}
}

func TestTrafficAdaptivoEnsanchaElSpacerYLoCuenta(t *testing.T) {
	tr := NewTraffic(Policy{MinInterval: 350 * time.Millisecond, Fast: 200 * time.Millisecond})
	var logs []string
	tr.SetLogger(func(f string, a ...any) { logs = append(logs, f) })

	tr.onRequest(gohoo.RequestEvent{StatusCode: 200})
	tr.onRequest(gohoo.RequestEvent{StatusCode: 429})
	if tr.Space.min != 400*time.Millisecond {
		t.Errorf("el Spacer debe usar el nuevo intervalo: %v", tr.Space.min)
	}
	st := tr.Stats()
	if st.Requests != 2 || st.RateLimited != 1 || st.Widened != 1 || st.Interval != 400*time.Millisecond {
		t.Fatalf("%+v", st)
	}
	if !strings.Contains(st.Summary(), "espaciado adaptivo 0.40 s (ensanchado 1 veces)") {
		t.Errorf("resumen: %q", st.Summary())
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "429") {
		t.Errorf("debe avisar del ensanchamiento: %v", logs)
	}
	tr.ResetStats() // cada corrida cuenta de cero, pero el intervalo aprendido se conserva
	if st := tr.Stats(); st.Widened != 0 || st.Requests != 0 || st.Interval != 400*time.Millisecond {
		t.Errorf("tras ResetStats: %+v", st)
	}
}

func TestResumenSinAdaptivoNoCambia(t *testing.T) {
	st := Stats{Requests: 3, RateLimited: 2, Retries: 2}
	if got := st.Summary(); got != "3 requests a Yahoo, 2 respuestas 429, 2 reintentos, 0 s de espera" {
		t.Errorf("%q", got)
	}
}

func TestTrafficAdaptivoConServidorQueLimita(t *testing.T) {
	tr, _ := newRelojTraffic(Policy{MinInterval: 350 * time.Millisecond, Fast: 200 * time.Millisecond,
		Retries429: 3, BackoffBase: time.Millisecond, BackoffMax: 2 * time.Millisecond})
	base, calls := chartServer(t, 2) // dos 429 y después responde
	if _, err := realClient(t, tr, base).Chart(context.Background(), "X", gohoo.ChartParams{Range: "1y"}); err != nil {
		t.Fatal(err)
	}
	st := tr.Stats()
	if calls.Load() != 3 || st.RateLimited != 2 {
		t.Fatalf("llamadas=%d %+v", calls.Load(), st)
	}
	if st.Widened != 1 || st.Interval != 400*time.Millisecond { // los dos 429 seguidos cuentan como un solo ensanchamiento
		t.Errorf("%+v", st)
	}
}
