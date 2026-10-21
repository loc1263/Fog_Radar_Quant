package tend

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"fogradar/internal/jobs"
)

func TestDiscoverUniverseUsaLaCacheDeSectores(t *testing.T) {
	path := filepath.Join(t.TempDir(), SectorCacheFile)

	f1 := &fakeYahoo{}
	u1, err := DiscoverUniverseOpts(context.Background(), f1, DiscoverOpts{Cache: LoadSectorCache(path, nil)}, quiet, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if got := f1.profiles.Load(); got != 32 { // 20 de Asia + 12 mega-cap de EEUU
		t.Errorf("primera corrida: %d quoteSummary, quería 32", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("la caché no se guardó: %v", err)
	}

	f2 := &fakeYahoo{}
	u2, err := DiscoverUniverseOpts(context.Background(), f2, DiscoverOpts{Cache: LoadSectorCache(path, nil)}, quiet, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if got := f2.profiles.Load(); got != 0 {
		t.Errorf("segunda corrida: %d quoteSummary, quería 0 (todo en caché)", got)
	}
	if len(u2.Asia) != len(u1.Asia) || len(u2.US) != len(u1.US) || len(u2.US) != 10 {
		t.Errorf("el universo cambió: Asia %d→%d, EEUU %d→%d", len(u1.Asia), len(u2.Asia), len(u1.US), len(u2.US))
	}
	for _, tk := range u2.US {
		if tk == "US10" || tk == "US11" {
			t.Errorf("%s debía excluirse por la sede guardada en la caché", tk)
		}
		if u2.Sector[tk] != "Technology" {
			t.Errorf("%s: sector %q, quería Technology", tk, u2.Sector[tk])
		}
	}
}

func TestDiscoverUniverseNoGuardaFallosEnLaCache(t *testing.T) {
	profileRetryWait = 0
	defer func() { profileRetryWait = 2 * time.Second }()
	path := filepath.Join(t.TempDir(), SectorCacheFile)

	f := &fakeYahoo{profileErr: errors.New("boom")}
	u, err := DiscoverUniverseOpts(context.Background(), f, DiscoverOpts{Cache: LoadSectorCache(path, nil)}, quiet, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.profiles.Load(); got != 64 { // 32 tickers × 2 intentos
		t.Errorf("%d intentos, quería 64", got)
	}
	if u.Sector[u.US[0]] != "Otro" {
		t.Errorf("sin respuesta de Yahoo el sector es 'Otro', fue %q", u.Sector[u.US[0]])
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("una consulta fallida no debe guardarse en la caché")
	}
}

// slowYahoo mide cuántos quoteSummary hay en vuelo a la vez.
type slowYahoo struct {
	fakeYahoo
	inflight, peak atomic.Int64
}

func (f *slowYahoo) QuoteSummary(ctx context.Context, ticker string, m []string) (map[string]any, error) {
	n := f.inflight.Add(1)
	for {
		p := f.peak.Load()
		if n <= p || f.peak.CompareAndSwap(p, n) {
			break
		}
	}
	time.Sleep(5 * time.Millisecond)
	f.inflight.Add(-1)
	return f.fakeYahoo.QuoteSummary(ctx, ticker, m)
}

func TestEnrichConsultaEnParaleloConLimite(t *testing.T) {
	f := &slowYahoo{}
	if _, err := DiscoverUniverseOpts(context.Background(), f, DiscoverOpts{Workers: 3}, quiet, quiet); err != nil {
		t.Fatal(err)
	}
	if got := f.profiles.Load(); got != 32 {
		t.Errorf("%d quoteSummary, quería 32", got)
	}
	if p := f.peak.Load(); p < 2 || p > 3 {
		t.Errorf("máximo en vuelo = %d, quería entre 2 y 3 (Workers=3)", p)
	}
}

func TestSectorCacheVigenciaYArchivo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", SectorCacheFile)
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	clock := now
	c := LoadSectorCache(path, func() time.Time { return clock })
	c.put("AAPL", "Technology", "United States")

	if e, ok := c.get("AAPL"); !ok || e.Sector != "Technology" || e.Country != "United States" {
		t.Fatalf("get = %+v, %v", e, ok)
	}
	if _, ok := c.get("MSFT"); ok {
		t.Error("un ticker desconocido no está en la caché")
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	clock = now.Add(SectorCacheTTL / 2)
	if _, ok := LoadSectorCache(path, func() time.Time { return clock }).get("AAPL"); !ok {
		t.Error("tras guardar y volver a leer, la entrada vigente debe seguir")
	}
	clock = now.Add(SectorCacheTTL + time.Hour)
	old := LoadSectorCache(path, func() time.Time { return clock })
	if _, ok := old.get("AAPL"); ok {
		t.Error("una entrada vencida no debe usarse")
	}
	old.put("MSFT", "Technology", "")
	if err := old.Save(); err != nil {
		t.Fatal(err)
	}
	if _, ok := old.m["AAPL"]; ok {
		t.Error("Save debe descartar las entradas vencidas")
	}
}

func TestSectorCacheArchivoDanadoYNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), SectorCacheFile)
	if err := os.WriteFile(path, []byte("{no es json"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := LoadSectorCache(path, nil)
	if _, ok := c.get("AAPL"); ok {
		t.Error("una caché dañada se trata como vacía")
	}
	c.put("AAPL", "Technology", "")
	if err := c.Save(); err != nil {
		t.Fatalf("debe poder sobrescribir el archivo dañado: %v", err)
	}

	var nilCache *SectorCache // sin caché: no hace nada ni falla
	nilCache.put("AAPL", "x", "y")
	if _, ok := nilCache.get("AAPL"); ok {
		t.Error("la caché nil no guarda nada")
	}
	if err := nilCache.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestForceDiscoveryIgnoraLaCacheDeSectores(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

	// Primera corrida: llena la caché.
	f1 := &fakeYahoo{}
	if _, err := DiscoverUniverseOpts(context.Background(), f1, discoverOpts(&jobs.Env{}, dir, nil, now), quiet, quiet); err != nil {
		t.Fatal(err)
	}
	// Sin la bandera, la segunda no consulta nada.
	f2 := &fakeYahoo{}
	if _, err := DiscoverUniverseOpts(context.Background(), f2, discoverOpts(&jobs.Env{}, dir, nil, now), quiet, quiet); err != nil {
		t.Fatal(err)
	}
	if got := f2.profiles.Load(); got != 0 {
		t.Fatalf("sin bandera: %d quoteSummary, quería 0", got)
	}
	// Con --force-discovery se consulta todo de nuevo.
	f3 := &fakeYahoo{}
	o := discoverOpts(&jobs.Env{}, dir, map[string]any{"force_discovery": true}, now)
	if !o.Cache.Refresh {
		t.Fatal("--force-discovery debe poner la caché en modo Refresh")
	}
	if _, err := DiscoverUniverseOpts(context.Background(), f3, o, quiet, quiet); err != nil {
		t.Fatal(err)
	}
	if got := f3.profiles.Load(); got != 32 {
		t.Errorf("con --force-discovery: %d quoteSummary, quería 32", got)
	}
}
