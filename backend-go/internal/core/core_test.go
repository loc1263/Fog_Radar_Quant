package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestSettingsDefaults(t *testing.T) {
	s, err := loadSettings(env(nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Env != "dev" || s.RateLimit != "300/minute" || s.Timezone != "America/Santiago" {
		t.Fatalf("defaults inesperados: %+v", s)
	}
	if len(s.Origins) != 1 || s.Origins[0] != "http://localhost:5173" {
		t.Fatalf("origins: %v", s.Origins)
	}
	if !filepath.IsAbs(s.DataDir) {
		t.Fatalf("DataDir debe ser absoluta: %s", s.DataDir)
	}
}

func TestSettingsProdExigeClaveFuerte(t *testing.T) {
	casos := map[string]map[string]string{
		"sin clave":     {"ENV": "prod", "ALLOWED_ORIGINS": "https://x.cl"},
		"corta":         {"ENV": "prod", "API_KEY": "abc", "ALLOWED_ORIGINS": "https://x.cl"},
		"de ejemplo":    {"ENV": "prod", "API_KEY": "cambiar-por-una-clave-larga-aleatoria", "ALLOWED_ORIGINS": "https://x.cl"},
		"poca entropía": {"ENV": "prod", "API_KEY": strings.Repeat("ab", 20), "ALLOWED_ORIGINS": "https://x.cl"},
		"localhost":     {"ENV": "prod", "API_KEY": "Zq8vN3xLm2Rt7YpKc9WdHs4JfBg6UaE1", "ALLOWED_ORIGINS": "http://localhost:5173"},
	}
	for nombre, vars := range casos {
		if _, err := loadSettings(env(vars), nil); err == nil {
			t.Errorf("%s: debía fallar", nombre)
		}
	}
	ok := map[string]string{"ENV": "prod", "API_KEY": "Zq8vN3xLm2Rt7YpKc9WdHs4JfBg6UaE1", "ALLOWED_ORIGINS": "https://x.cl,https://y.cl"}
	s, err := loadSettings(env(ok), nil)
	if err != nil || !s.IsProd() || len(s.Origins) != 2 {
		t.Fatalf("prod válido: %v %+v", err, s)
	}
}

func TestDotenvYPrioridad(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(f, []byte("\xef\xbb\xbf# comentario\nENV=test\nAPP_NAME=\"Mi App\"\nLOG_LEVEL=debug # nota\nexport TIMEZONE=UTC\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := loadSettings(env(map[string]string{"TIMEZONE": "America/Santiago"}), []string{f})
	if err != nil {
		t.Fatal(err)
	}
	if s.Env != "test" || s.AppName != "Mi App" || s.LogLevel != "DEBUG" {
		t.Fatalf("dotenv mal leído: %+v", s)
	}
	if s.Timezone != "America/Santiago" {
		t.Fatal("la variable de entorno debe ganar al .env")
	}
}

func TestKeyValid(t *testing.T) {
	s := &Settings{}
	if !s.KeyValid("") {
		t.Fatal("sin API_KEY configurada se acepta todo")
	}
	s.APIKey = "secreta-ñ"
	if !s.KeyValid("secreta-ñ") || s.KeyValid("") || s.KeyValid("otra") || s.KeyValid("secreta-ñ ") {
		t.Fatal("comparación de clave incorrecta")
	}
}

func TestParseLimit(t *testing.T) {
	n, d, err := ParseLimit("300/minute")
	if err != nil || n != 300 || d != time.Minute {
		t.Fatal(n, d, err)
	}
	if _, d, _ := ParseLimit("5/seconds"); d != time.Second {
		t.Fatal("plural")
	}
	for _, bad := range []string{"", "300", "0/minute", "x/minute", "5/year"} {
		if _, _, err := ParseLimit(bad); err == nil {
			t.Errorf("%q debía fallar", bad)
		}
	}
}

func TestLimiterVentanaDeslizante(t *testing.T) {
	l := NewLimiter()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if !l.Hit(3, time.Minute, "s", "ip") {
			t.Fatalf("hit %d debía pasar", i)
		}
	}
	if l.Hit(3, time.Minute, "s", "ip") {
		t.Fatal("el 4º debía rechazarse")
	}
	if !l.Hit(3, time.Minute, "s", "otra-ip") || !l.Hit(3, time.Minute, "otro-scope", "ip") {
		t.Fatal("clave y scope son independientes")
	}
	now = now.Add(61 * time.Second)
	if !l.Hit(3, time.Minute, "s", "ip") {
		t.Fatal("pasada la ventana debe volver a aceptar")
	}
}

func TestCheckEnTestNoLimitaSalvoQueSePida(t *testing.T) {
	l := NewLimiter()
	r := httptest.NewRequest("GET", "/", nil)
	s := &Settings{Env: "test"}
	for i := 0; i < 5; i++ {
		if e := l.Check(s, "1/minute", "x", r); e != nil {
			t.Fatal("en test no se limita")
		}
	}
	s.RateLimitInTests = true
	if e := l.Check(s, "1/minute", "y", r); e != nil {
		t.Fatal(e)
	}
	e := l.Check(s, "1/minute", "y", r)
	if e == nil || e.Status != 429 || e.Code != "rate_limited" {
		t.Fatalf("esperaba 429: %v", e)
	}
}

func TestClientKey(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:4444"
	if ClientKey(r) != "10.0.0.5" {
		t.Fatal(ClientKey(r))
	}
	r.Header.Set("X-Forwarded-For", " 203.0.113.7 , 10.0.0.5")
	if ClientKey(r) != "203.0.113.7" {
		t.Fatal(ClientKey(r))
	}
}

func TestGate(t *testing.T) {
	g := NewGate(1)
	rel, err := g.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Acquire(); err == nil || err.Status != http.StatusServiceUnavailable || err.Code != "busy" {
		t.Fatalf("esperaba 503 busy: %v", err)
	}
	rel()
	if _, err := g.Acquire(); err != nil {
		t.Fatal("tras liberar debe haber cupo")
	}
}

func TestFormatISOSinFraccion(t *testing.T) {
	scl, _ := time.LoadLocation("America/Santiago")
	casos := []struct {
		t    time.Time
		want string
	}{
		{time.Date(2026, 10, 8, 0, 46, 24, 252896000, time.UTC), "2026-10-08T00:46:24.252896Z"},
		{time.Date(2026, 10, 8, 0, 46, 24, 0, time.UTC), "2026-10-08T00:46:24Z"},
		{time.Date(2026, 7, 25, 16, 16, 46, 0, scl), "2026-07-25T16:16:46-04:00"},
		{time.Date(2026, 9, 24, 21, 30, 19, 0, scl), "2026-09-24T21:30:19-03:00"},
	}
	for _, c := range casos {
		if got := FormatISO(c.t); got != c.want {
			t.Errorf("got %s want %s", got, c.want)
		}
		back, err := ParseISO(c.want, scl)
		if err != nil || !back.Equal(c.t) {
			t.Errorf("ida y vuelta de %s: %v %v", c.want, back, err)
		}
	}
	naive, err := ParseISO("2026-09-24T21:30:19", scl)
	if err != nil || FormatISO(naive) != "2026-09-24T21:30:19-03:00" {
		t.Errorf("fecha sin zona debe tomar la de Settings: %v %v", naive, err)
	}
}

func TestJobRegistry(t *testing.T) {
	dir := t.TempDir()
	lines := []string{
		`{"id":"a","module":"etf","status":"running","trigger":"cli","started_at":"2026-10-01T10:00:00Z"}`,
		`{"id":"a","module":"etf","status":"success","trigger":"cli","started_at":"2026-10-01T10:00:00Z","finished_at":"2026-10-01T10:05:00Z","duration_s":300.0,"message":"ok"}`,
		`basura que no es json`,
		`{"id":"b","module":"etf","status":"error","started_at":"2026-10-02T10:00:00Z","message":"boom"}`,
		`{"id":"c","module":"etf","status":"running","trigger":"cron","started_at":"2026-10-03T10:00:00Z"}`,
	}
	if err := os.WriteFile(filepath.Join(dir, "etf.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := NewJobRegistry(dir)
	r.Now = func() time.Time { return time.Date(2026, 10, 3, 10, 30, 0, 0, time.UTC) }

	h := r.History("etf", 10)
	if len(h) != 3 || h[0].ID != "c" || h[1].ID != "b" || h[2].ID != "a" {
		t.Fatalf("orden/dedupe: %+v", h)
	}
	if h[0].Status != "running" || h[1].Trigger != "cli" || h[2].Status != "success" {
		t.Fatalf("estados: %+v", h)
	}
	if ls := r.LastSuccess("etf"); ls == nil || ls.ID != "a" {
		t.Fatalf("LastSuccess: %+v", ls)
	}
	r.Now = func() time.Time { return time.Date(2026, 10, 3, 17, 0, 0, 0, time.UTC) }
	if l := r.Latest("etf"); l == nil || l.Status != "stale" {
		t.Fatalf("una corrida running de >6 h está colgada: %+v", l)
	}
	if r.Latest("nada") != nil || len(r.History("nada", 5)) != 0 {
		t.Fatal("módulo sin registro")
	}
}

func TestProgramas(t *testing.T) {
	if Programas[0].Cron() != "0 6 * * 1" || Programas[1].Cron() != "0 18 * * 1-5" {
		t.Fatal(Programas[0].Cron(), Programas[1].Cron())
	}
	if Programas[0].Descripcion() != "los lunes a las 06:00" || Programas[1].Descripcion() != "de lunes a viernes a las 18:00" {
		t.Fatal(Programas[0].Descripcion(), Programas[1].Descripcion())
	}
	scl, _ := time.LoadLocation("America/Santiago")
	viernes := time.Date(2026, 10, 9, 18, 0, 0, 0, scl) // viernes 18:00 exactos → estrictamente posterior
	if got := Programas[1].Proxima(viernes); got.Weekday() != time.Monday || got.Hour() != 18 {
		t.Fatalf("tras el viernes 18:00 viene el lunes: %v", got)
	}
	if got := Programas[0].Proxima(viernes); got.Weekday() != time.Monday || got.Hour() != 6 {
		t.Fatalf("semanal: %v", got)
	}
}
