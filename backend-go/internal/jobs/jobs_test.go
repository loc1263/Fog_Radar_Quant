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

func TestSecuenciaEnOrdenYPausaEntreModulos(t *testing.T) {
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
	if strings.Join(orden, ",") != "a1,a2,a3" || len(dormido) != 2 {
		t.Fatalf("orden=%v pausas=%v", orden, dormido)
	}
	for _, r := range res {
		if r.Estado != EstadoOK || !strings.HasPrefix(r.Mensaje, "ok ") {
			t.Fatalf("%+v", r)
		}
	}
	// queda en el registro de jobs: success con duración
	rec := core.NewJobRegistry(env.S.JobsDir()).Latest("a2")
	if rec == nil || rec.Status != "success" || rec.DurationS == nil || rec.FinishedAt == nil || rec.Trigger != "cli" {
		t.Fatalf("%+v", rec)
	}
}

func TestFallaYSigueConElSiguienteYAlerta(t *testing.T) {
	env := testEnv(t)
	reg("b1", func(context.Context, map[string]any) (string, error) { return "", errors.New("boom") })
	reg("b2", func(context.Context, map[string]any) (string, error) { return "ok", nil })
	var alerta string
	cero := 0
	res, err := CorrerSecuencia(context.Background(), env, []string{"b1", "b2"},
		SecuenciaOpts{Reintentos: &cero, Pausa: new(time.Duration), Sleep: func(time.Duration) {}, Alert: func(s string) { alerta = s }, Trigger: "cron"})
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Estado != EstadoError || res[1].Estado != EstadoOK {
		t.Fatalf("%+v", res)
	}
	if !strings.Contains(alerta, "b1 (boom)") || !strings.Contains(alerta, "(cron)") {
		t.Fatalf("alerta: %q", alerta)
	}
	rec := core.NewJobRegistry(env.S.JobsDir()).Latest("b1")
	if rec.Status != "error" || rec.Message == nil || *rec.Message != "boom" || rec.Trigger != "cron" {
		t.Fatalf("%+v", rec)
	}
}

func TestReintentaConEspera(t *testing.T) {
	env := testEnv(t)
	n := 0
	reg("c1", func(context.Context, map[string]any) (string, error) {
		n++
		if n < 3 {
			return "", errors.New("transitorio")
		}
		return "al tercero", nil
	})
	var esperas []time.Duration
	dos, espera := 2, 10*time.Minute
	res, _ := CorrerSecuencia(context.Background(), env, []string{"c1"},
		SecuenciaOpts{Reintentos: &dos, EsperaRein: &espera, Sleep: func(d time.Duration) { esperas = append(esperas, d) }})
	if res[0].Estado != EstadoOK || res[0].Intentos != 3 || len(esperas) != 2 || esperas[0] != espera {
		t.Fatalf("%+v %v", res[0], esperas)
	}
	h := core.NewJobRegistry(env.S.JobsDir()).History("c1", 10)
	if len(h) != 3 || h[0].Status != "success" || h[1].Status != "error" {
		t.Fatalf("cada intento queda en el registro: %+v", h)
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
	if err := r.Append(core.JobRecord{ID: "x", Module: "f1", Status: "running", Trigger: "cli", StartedAt: core.Fecha{Time: time.Now().UTC()}}); err != nil {
		t.Fatal(err)
	}
	cero := 0
	res, _ := CorrerSecuencia(context.Background(), env, []string{"f1"}, SecuenciaOpts{Reintentos: &cero})
	if res[0].Estado != EstadoError || !strings.Contains(res[0].Mensaje, "en curso") {
		t.Fatalf("%+v", res[0])
	}
}

func TestSinPipelinePortado(t *testing.T) {
	env := testEnv(t)
	cero := 0
	res, _ := CorrerSecuencia(context.Background(), env, []string{"no_existe"}, SecuenciaOpts{Reintentos: &cero})
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
		if err := r.Append(core.JobRecord{ID: string(rune('a'+i%26)) + strings.Repeat("0", 3) + time.Duration(i).String(), Module: "m", Status: "success", Trigger: "cli",
			StartedAt: core.Fecha{Time: time.Unix(int64(1_700_000_000+i), 0).UTC()}, Message: &msg}); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(r.History("m", 5000)); n > core.MaxLineas || n < 100 {
		t.Fatalf("tras rotar deben quedar a lo más %d líneas, hay %d", core.MaxLineas, n)
	}
}
