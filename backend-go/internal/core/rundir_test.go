package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunsEstandar(t *testing.T) {
	s := &Settings{DataDir: t.TempDir(), Loc: time.UTC}
	const m = ModuleKey("etf")
	t0 := time.Date(2026, 10, 21, 18, 4, 16, 0, time.UTC)

	a, err := NewRun(s, m, t0)
	if err != nil || filepath.Base(a) != "20261021_180416" {
		t.Fatalf("id: %v %v", a, err)
	}
	if LatestRun(s, m) != "" {
		t.Fatal("una corrida sin run.json no cuenta")
	}
	b, _ := NewRun(s, m, t0) // misma hora: sufijo _2
	c, _ := NewRun(s, m, t0.Add(time.Hour))
	if filepath.Base(b) != "20261021_180416_2" || filepath.Base(c) != "20261021_190416" {
		t.Fatalf("sufijo: %v %v", b, c)
	}
	for _, d := range []string{a, b, c} {
		if err := FinishRun(d, m, t0, t0.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	runs := ListRuns(s, m)
	if len(runs) != 3 || runs[0] != c || runs[1] != b || runs[2] != a {
		t.Fatalf("orden (más reciente primero): %v", runs)
	}
	if LatestRun(s, m) != c || PreviousRun(s, m, c) != b || PreviousRun(s, m, a) != "" {
		t.Fatal("última y anterior")
	}
	if tm := RunTime(b, time.UTC); tm == nil || !tm.Equal(t0) {
		t.Fatalf("RunTime: %v", tm)
	}
	if err := os.Remove(filepath.Join(c, RunMarker)); err != nil || LatestRun(s, m) != b {
		t.Fatal("una corrida a medias se ignora")
	}
	if s.CacheDir(m) != filepath.Join(s.DataDir, "etf", "cache") || s.LocksDir() != filepath.Join(s.DataDir, "_sistema", "locks") {
		t.Fatal("rutas")
	}
}
