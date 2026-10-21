package core

import (
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
	if s.Env != "dev" || s.Timezone != "America/Santiago" {
		t.Fatalf("defaults inesperados: %+v", s)
	}
	if !filepath.IsAbs(s.DataDir) {
		t.Fatalf("DataDir debe ser absoluta: %s", s.DataDir)
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
		`{"id":"a","module":"etf","status":"running","started_at":"2026-10-01T10:00:00Z"}`,
		`{"id":"a","module":"etf","status":"success","started_at":"2026-10-01T10:00:00Z","finished_at":"2026-10-01T10:05:00Z","duration_s":300.0,"message":"ok"}`,
		`basura que no es json`,
		`{"id":"b","module":"etf","status":"error","started_at":"2026-10-02T10:00:00Z","message":"boom"}`,
		`{"id":"c","module":"etf","status":"running","started_at":"2026-10-03T10:00:00Z"}`,
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
	if h[0].Status != "running" || h[2].Status != "success" {
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
	if len(Programas) != 1 {
		t.Fatalf("hay un único programa diario: %d", len(Programas))
	}
	p := Programas[0]
	if p.Cron() != "0 18 * * *" || p.Descripcion() != "todos los días a las 18:00" {
		t.Fatal(p.Cron(), p.Descripcion())
	}
	if len(p.Modulos) != 4 {
		t.Fatalf("módulos: %v", p.Modulos)
	}
	scl, _ := time.LoadLocation("America/Santiago")
	sabado := time.Date(2026, 10, 10, 18, 0, 0, 0, scl) // 18:00 exactos → la próxima es estrictamente posterior
	if got := p.Proxima(sabado); got.Weekday() != time.Sunday || got.Hour() != 18 {
		t.Fatalf("tras el sábado 18:00 viene el domingo: %v", got)
	}
	if got := p.Anterior(sabado.Add(time.Hour)); !got.Equal(sabado) {
		t.Fatalf("anterior: %v", got)
	}
}
