package retencion

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fogradar/internal/core"
)

func nuevaCorrida(t *testing.T, s *core.Settings, m core.ModuleKey, cuando time.Time, terminada bool) string {
	t.Helper()
	dir, err := core.NewRun(s, m, cuando)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "datos.csv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if terminada {
		if err := core.FinishRun(dir, m, cuando, cuando); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func existe(p string) bool { _, err := os.Stat(p); return err == nil }

func TestPodaCorridasViejasYConservaLaVigente(t *testing.T) {
	s := &core.Settings{DataDir: t.TempDir(), Loc: time.UTC}
	ahora := time.Date(2026, 10, 21, 18, 0, 0, 0, time.UTC)
	const m = core.ModuleKey("etf")

	vieja := nuevaCorrida(t, s, m, ahora.AddDate(0, 0, -30), true)
	limite := nuevaCorrida(t, s, m, ahora.AddDate(0, 0, -6), true) // dentro de los 7 días
	fuera := nuevaCorrida(t, s, m, ahora.AddDate(0, 0, -7), true)  // el 8.º día hacia atrás: se borra
	vigente := nuevaCorrida(t, s, m, ahora, true)
	incompletaVieja := nuevaCorrida(t, s, m, ahora.AddDate(0, 0, -2), false)
	enCurso := nuevaCorrida(t, s, m, ahora.Add(-time.Hour), false) // corriendo ahora: no se toca
	cache := filepath.Join(s.CacheDir(m), "info_cache.json")
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Podar(s, 7, ahora, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Eliminados) != 3 || r.Bytes == 0 {
		t.Fatalf("eliminó %d: %+v", len(r.Eliminados), r.Eliminados)
	}
	for _, d := range []string{vieja, fuera, incompletaVieja} {
		if existe(d) {
			t.Errorf("%s debía borrarse", filepath.Base(d))
		}
	}
	for _, d := range []string{limite, vigente, enCurso, cache} {
		if !existe(d) {
			t.Errorf("%s debía quedar", filepath.Base(d))
		}
	}
}

func TestSiTodoEsViejoQuedaLaUltimaYSimularDiasCero(t *testing.T) {
	s := &core.Settings{DataDir: t.TempDir(), Loc: time.UTC}
	ahora := time.Date(2026, 10, 21, 18, 0, 0, 0, time.UTC)
	const m = core.ModuleKey("tendencia")
	a := nuevaCorrida(t, s, m, ahora.AddDate(0, 0, -40), true)
	b := nuevaCorrida(t, s, m, ahora.AddDate(0, 0, -30), true)

	if r, _ := Podar(s, 7, ahora, true); len(r.Eliminados) != 1 || !existe(a) {
		t.Fatalf("simulación: %+v", r)
	}
	if r, _ := Podar(s, 0, ahora, false); len(r.Eliminados) != 0 || !existe(a) {
		t.Fatalf("dias=0 desactiva: %+v", r)
	}
	if _, err := Podar(s, 7, ahora, false); err != nil || existe(a) || !existe(b) {
		t.Fatal("queda siempre la corrida más reciente, aunque sea vieja")
	}
}
