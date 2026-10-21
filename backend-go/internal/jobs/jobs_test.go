package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"fogradar/internal/core"
	"fogradar/internal/yahoo"
)

func testEnv(t *testing.T) *Env {
	t.Helper()
	loc, _ := time.LoadLocation("America/Santiago")
	s := &core.Settings{Env: "test", DataDir: t.TempDir(), Loc: loc}
	tr := yahoo.NewTraffic(yahoo.Policy{})
	return &Env{S: s, Traffic: tr, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func reg(name string, run func(ctx context.Context, opts map[string]any) (string, error)) {
	Register(name, Pipeline{Run: func(ctx context.Context, _ *Env, o map[string]any) (string, error) { return run(ctx, o) }})
}

func TestSecuenciaEnOrdenYSinPausaSiNoHubo429(t *testing.T) {
	env := testEnv(t)
	var orden []string
	for _, m := range []string{"a1", "a2", "a3"} {
		reg(m, func(context.Context, map[string]any) (string, error) { orden = append(orden, m); return "ok " + m, nil })
	}
	var dormido []time.Duration
	pausa := 7 * time.Second
	res, err := CorrerSecuencia(context.Background(), env, []string{"a1", "a2", "a3"},
		SecuenciaOpts{Pausa: &pausa, Sleep: func(d time.Duration) { dormido = append(dormido, d) }})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(orden, ",") != "a1,a2,a3" || len(dormido) != 0 { // tráfico limpio: no se duerme entre módulos
		t.Fatalf("orden=%v pausas=%v", orden, dormido)
	}
	for _, r := range res {
		if r.Estado != EstadoOK || !strings.HasPrefix(r.Mensaje, "ok ") {
			t.Fatalf("%+v", r)
		}
	}
	// queda en el registro de jobs: success con duración
	rec := core.NewJobRegistry(env.S.JobsDir()).Latest("a2")
	if rec == nil || rec.Status != "success" || rec.DurationS == nil || rec.FinishedAt == nil {
		t.Fatalf("%+v", rec)
	}
}

func TestPausaTrasUnModuloQueFalla(t *testing.T) {
	env := testEnv(t)
	reg("p1", func(context.Context, map[string]any) (string, error) { return "", errors.New("boom") })
	reg("p2", func(context.Context, map[string]any) (string, error) { return "ok", nil })
	reg("p3", func(context.Context, map[string]any) (string, error) { return "ok", nil })
	var dormido []time.Duration
	pausa := 7 * time.Second
	_, err := CorrerSecuencia(context.Background(), env, []string{"p1", "p2", "p3"},
		SecuenciaOpts{Pausa: &pausa, Sleep: func(d time.Duration) { dormido = append(dormido, d) }})
	if err != nil {
		t.Fatal(err)
	}
	if len(dormido) != 1 || dormido[0] != pausa { // solo tras p1 (falló); p2 → p3 fue limpio
		t.Fatalf("pausas = %v, quería [7s]", dormido)
	}
}

func TestNecesitaPausa(t *testing.T) {
	ok, mal := Resultado{Estado: EstadoOK}, Resultado{Estado: EstadoError}
	casos := []struct {
		nombre string
		r      Resultado
		st     yahoo.Stats
		want   bool
	}{
		{"tráfico limpio", ok, yahoo.Stats{Requests: 900}, false},
		{"sin requests", ok, yahoo.Stats{}, false},
		{"hubo 429", ok, yahoo.Stats{Requests: 50, RateLimited: 1}, true},
		{"hubo reintentos", ok, yahoo.Stats{Requests: 50, Retries: 2}, true},
		{"el módulo falló", mal, yahoo.Stats{Requests: 10}, true},
	}
	for _, c := range casos {
		if got := necesitaPausa(c.r, c.st); got != c.want {
			t.Errorf("%s: necesitaPausa = %v, quería %v", c.nombre, got, c.want)
		}
	}
	if !strings.Contains(motivoPausa(mal, yahoo.Stats{}), "falló") ||
		!strings.Contains(motivoPausa(ok, yahoo.Stats{RateLimited: 3}), "3 respuestas 429") {
		t.Error("el motivo de la pausa debe decir por qué se espera")
	}
}

func TestFallaYSigueConElSiguiente(t *testing.T) {
	env := testEnv(t)
	reg("b1", func(context.Context, map[string]any) (string, error) { return "", errors.New("boom") })
	reg("b2", func(context.Context, map[string]any) (string, error) { return "ok", nil })
	res, err := CorrerSecuencia(context.Background(), env, []string{"b1", "b2"},
		SecuenciaOpts{Pausa: new(time.Duration), Sleep: func(time.Duration) {}})
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Estado != EstadoError || res[1].Estado != EstadoOK {
		t.Fatalf("%+v", res)
	}
	rec := core.NewJobRegistry(env.S.JobsDir()).Latest("b1")
	if rec.Status != "error" || rec.Message == nil || *rec.Message != "boom" {
		t.Fatalf("%+v", rec)
	}
}

func TestOmitirYOpcionesPorModulo(t *testing.T) {
	env := testEnv(t)
	var got map[string]any
	reg("d1", func(_ context.Context, o map[string]any) (string, error) { got = o; return "", nil })
	reg("d2", func(context.Context, map[string]any) (string, error) { t.Fatal("no debía correr"); return "", nil })
	res, _ := CorrerSecuencia(context.Background(), env, []string{"d1", "d2"}, SecuenciaOpts{
		Opciones: map[string]map[string]any{"d1": {"solo_scoring": true}}, Omitir: map[string]string{"d2": "sin modo offline"},
		Pausa: new(time.Duration), Sleep: func(time.Duration) {}})
	if got["solo_scoring"] != true || res[1].Estado != EstadoOmitido || res[1].Mensaje != "sin modo offline" {
		t.Fatalf("%v %+v", got, res)
	}
}

func TestLockGlobalOcupado(t *testing.T) {
	env := testEnv(t)
	reg("e1", func(context.Context, map[string]any) (string, error) { return "", nil })
	unlock, err := core.LockPipelines(env.S, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := CorrerSecuencia(context.Background(), env, []string{"e1"}, SecuenciaOpts{}); !errors.Is(err, core.ErrOcupado) {
		t.Fatalf("esperaba ErrOcupado: %v", err)
	}
	unlock()
	if _, err := CorrerSecuencia(context.Background(), env, []string{"e1"}, SecuenciaOpts{Pausa: new(time.Duration)}); err != nil {
		t.Fatalf("liberado el lock debe correr: %v", err)
	}
}

func TestNoCorreSiElModuloYaEstaEnCurso(t *testing.T) {
	env := testEnv(t)
	reg("f1", func(context.Context, map[string]any) (string, error) { t.Fatal("no debía correr"); return "", nil })
	r := core.NewJobRegistry(env.S.JobsDir())
	if err := r.Append(core.JobRecord{ID: "x", Module: "f1", Status: "running", StartedAt: core.Fecha{Time: time.Now().UTC()}}); err != nil {
		t.Fatal(err)
	}
	res, _ := CorrerSecuencia(context.Background(), env, []string{"f1"}, SecuenciaOpts{})
	if res[0].Estado != EstadoError || !strings.Contains(res[0].Mensaje, "en curso") {
		t.Fatalf("%+v", res[0])
	}
}

func TestSinPipelinePortado(t *testing.T) {
	env := testEnv(t)
	res, _ := CorrerSecuencia(context.Background(), env, []string{"no_existe"}, SecuenciaOpts{})
	if res[0].Estado != EstadoError {
		t.Fatalf("%+v", res[0])
	}
}

func TestOptionConvert(t *testing.T) {
	o := Option{Flag: "--top-n", Kind: Int, Min: 2, Max: 60}
	if v, err := o.Convert("20"); err != nil || v != 20 {
		t.Fatal(v, err)
	}
	for _, bad := range []string{"1", "61", "x"} {
		if _, err := o.Convert(bad); err == nil {
			t.Errorf("%q debía fallar", bad)
		}
	}
	f := Option{Flag: "--max-weight", Kind: Float, Min: 0.01, Max: 1}
	if v, err := f.Convert("0.1"); err != nil || v != 0.1 {
		t.Fatal(v, err)
	}
	if (Option{Flag: "--solo-scoring"}).Dest() != "solo_scoring" || (Option{Flag: "--x", Key: "kw"}).Dest() != "kw" {
		t.Fatal("Dest")
	}
}

func TestRegistroRotaAlPasarElLimite(t *testing.T) {
	dir := t.TempDir()
	r := core.NewJobRegistry(dir)
	msg := strings.Repeat("x", 600)
	for i := 0; i < 1500; i++ {
		if err := r.Append(core.JobRecord{ID: string(rune('a'+i%26)) + strings.Repeat("0", 3) + time.Duration(i).String(), Module: "m", Status: "success",
			StartedAt: core.Fecha{Time: time.Unix(int64(1_700_000_000+i), 0).UTC()}, Message: &msg}); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(r.History("m", 5000)); n > core.MaxLineas || n < 100 {
		t.Fatalf("tras rotar deben quedar a lo más %d líneas, hay %d", core.MaxLineas, n)
	}
}
