package tend

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/loc1263/gohoo"

	"fogradar/internal/jobs"
	"fogradar/internal/yahoo"
)

func TestNormName(t *testing.T) {
	cases := map[string]string{
		"Big Bank Corp":                    "BIG BANK", // «CO» no empata dentro de «CORP»
		"Alibaba Group Holding Limited":    "ALIBABA",
		"Tencent Holdings Ltd. (ADR)":      "TENCENT (ADR)", // \b no separa « (» de «(ADR)»,
		"Samsung Electronics Co., Ltd.":    "SAMSUNG ELECTRONICS",
		"ÉCO Power":                        "ÉCO POWER", // «É» es letra: no hay límite de palabra antes de CO
		"Toyota Motor Corporation Class A": "TOYOTA MOTOR",
	}
	for in, want := range cases {
		if got := normName(in); got != want {
			t.Errorf("normName(%q) = %q, quería %q", in, got, want)
		}
	}
	if got := normName("Un Nombre Larguisimo De Empresa Sin Sufijo"); len([]rune(got)) != 20 {
		t.Errorf("no recortó a 20 caracteres: %q", got)
	}
}

func TestNormalizeQuote(t *testing.T) {
	ok := normalizeQuote(map[string]any{"symbol": "AAPL", "shortName": "Apple", "marketCap": 3e12, "currency": "USD", "quoteType": "EQUITY"}, "EEUU")
	if ok == nil || ok.Name != "Apple" || ok.MarketCap != 3e12 || ok.Sector != "Otro" {
		t.Fatalf("%+v", ok)
	}
	// Envoltorio {raw, fmt} y nombre largo con preferencia.
	w := normalizeQuote(map[string]any{"symbol": "X", "longName": "Largo", "shortName": "C", "intradaymarketcap": map[string]any{"raw": 5.0, "fmt": "5"}, "currency": "JPY"}, "Japón")
	if w == nil || w.Name != "Largo" || w.MarketCap != 5 {
		t.Fatalf("%+v", w)
	}
	for name, q := range map[string]map[string]any{
		"sin símbolo":   {"marketCap": 1.0, "currency": "USD"},
		"sin marketcap": {"symbol": "A", "currency": "USD"},
		"sin moneda":    {"symbol": "A", "marketCap": 1.0},
		"no es acción":  {"symbol": "A", "marketCap": 1.0, "currency": "USD", "quoteType": "ETF"},
	} {
		if normalizeQuote(q, "x") != nil {
			t.Errorf("%s: debía descartarse", name)
		}
	}
}

func TestDedupeYSeleccion(t *testing.T) {
	cs := []*candidate{
		{Symbol: "A.SS", Name: "Bank of X Co Ltd", MarketCap: 5},
		{Symbol: "B", Name: "Otra", MarketCap: 9},
		{Symbol: "A.HK", Name: "Bank of X Limited", MarketCap: 8}, // mismo grupo, mayor cap: gana
	}
	d := dedupeByCompany(cs)
	if len(d) != 2 || d[0].Symbol != "A.HK" || d[1].Symbol != "B" {
		t.Fatalf("dedupe: %v %v", d[0].Symbol, len(d))
	}
	top := selectCandidates(d, 1, 0)
	if len(top) != 1 || top[0].Symbol != "B" {
		t.Fatalf("top-N: %+v", top)
	}
	mega := selectCandidates(d, 0, 9)
	if len(mega) != 1 || mega[0].Symbol != "B" {
		t.Fatalf("mega-cap: %+v", mega)
	}
}

func TestExcludeForeignHQ(t *testing.T) {
	us, tw := "United States", "Taiwan"
	cs := []*candidate{{Symbol: "AAPL", HQCountry: &us}, {Symbol: "TSM", HQCountry: &tw}, {Symbol: "XYZ"}}
	var msg string
	out := excludeForeignHQ(cs, func(f string, a ...any) { msg = fmt.Sprintf(f, a...) })
	if len(out) != 2 || out[0].Symbol != "AAPL" || out[1].Symbol != "XYZ" { // sin sede conocida se conserva
		t.Fatalf("%+v", out)
	}
	if !strings.Contains(msg, "TSM (Taiwan)") {
		t.Fatalf("log: %q", msg)
	}
}

func TestSnapshotAgeDays(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.UTC)
	for name, want := range map[string]int{"universe_20261009.json": 0, "universe_20261007.json": 2, "universe_20261005.json": 4} {
		if got, ok := snapshotAgeDays(name, now); !ok || got != want {
			t.Errorf("%s: %d %v", name, got, ok)
		}
	}
	if _, ok := snapshotAgeDays("universe_x.json", now); ok {
		t.Error("nombre inválido debía fallar")
	}
}

// fakeYahoo cumple yahoo.Source y Screener sin red: 12 empresas por región de Asia y 12 mega-cap de EEUU.
type fakeYahoo struct {
	screenErr  error
	screens    int
	profiles   atomic.Int64 // QuoteSummary corre en paralelo
	profileErr error        // si no es nil, todo quoteSummary falla
}

func (f *fakeYahoo) Chart(context.Context, string, gohoo.ChartParams) (*gohoo.ChartResult, error) {
	return nil, nil
}

func (f *fakeYahoo) Fundamentals(context.Context, string, []string) (map[string]any, error) {
	return nil, nil
}

func (f *fakeYahoo) Screen(_ context.Context, p yahoo.ScreenParams) (yahoo.Quotes, error) {
	f.screens++
	if f.screenErr != nil {
		return nil, f.screenErr
	}
	var q struct{ Operands []any }
	if err := json.Unmarshal([]byte(p.Query.String()), &q); err != nil {
		return nil, err
	}
	region := q.Operands[1].(string)
	var qs yahoo.Quotes
	for i := range 12 {
		cap := 3e11 + float64(i)*1e9
		cur := "USD"
		switch region {
		case "jp":
			cur = "JPY"
		case "hk":
			cur = "HKD"
		}
		if region != "us" {
			cap = 1e10 + float64(i)*1e9
		}
		qs = append(qs, map[string]any{"symbol": fmt.Sprintf("%s%d", strings.ToUpper(region), i), "longName": fmt.Sprintf("Empresa %s %d", region, i),
			"marketCap": cap, "currency": cur, "quoteType": "EQUITY"})
	}
	return qs, nil
}

func (f *fakeYahoo) QuoteSummary(_ context.Context, ticker string, _ []string) (map[string]any, error) {
	f.profiles.Add(1)
	if f.profileErr != nil {
		return nil, f.profileErr
	}
	country := "United States"
	if ticker == "US10" || ticker == "US11" { // ADR de empresa extranjera
		country = "Taiwan"
	}
	return map[string]any{"assetProfile": map[string]any{"sector": "Technology", "country": country}}, nil
}

func quiet(string, ...any) {}

func TestDiscoverUniverse(t *testing.T) {
	f := &fakeYahoo{}
	u, err := DiscoverUniverseOpts(context.Background(), f, DiscoverOpts{}, quiet, quiet)
	if err != nil {
		t.Fatal(err)
	}
	if f.screens != 7 { // 6 regiones de Asia + EEUU
		t.Errorf("screens = %d", f.screens)
	}
	if len(u.Asia) != 20 || len(u.US) != 10 { // 6x12 → top 20; 12 mega-cap − 2 ADR extranjeros
		t.Errorf("Asia=%d US=%d", len(u.Asia), len(u.US))
	}
	if u.Source != "dynamic_yahoo_screener" || u.Sector[u.US[0]] != "Technology" {
		t.Errorf("%s %v", u.Source, u.Sector[u.US[0]])
	}
	for _, tk := range u.US {
		if tk == "US10" || tk == "US11" {
			t.Errorf("%s debía excluirse por sede", tk)
		}
	}
	if u.Info[u.US[0]].Country != "EEUU" {
		t.Errorf("%+v", u.Info[u.US[0]])
	}
}

func TestDiscoverUniverseInsuficiente(t *testing.T) {
	screenBackoff = 0
	defer func() { screenBackoff = 3 * time.Second }()
	f := &fakeYahoo{screenErr: yahoo.ErrEmptyScreen}
	if _, err := DiscoverUniverseOpts(context.Background(), f, DiscoverOpts{}, quiet, quiet); err == nil {
		t.Fatal("debía fallar")
	}
}

func writeSnapshot(t *testing.T, dir string, edadDias int, now time.Time) string {
	t.Helper()
	u := FallbackUniverse()
	d := now.AddDate(0, 0, -edadDias)
	path, err := SaveSnapshot(dir, u, "dynamic_yahoo_screener", d)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveUniverseEdad(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

	t.Run("reciente: se reutiliza sin screener", func(t *testing.T) {
		dir := t.TempDir()
		writeSnapshot(t, dir, 3, now)
		f := &fakeYahoo{}
		u, err := resolveUniverse(context.Background(), &jobs.Env{Yahoo: f}, dir, nil, now, quiet, quiet)
		if err != nil || f.screens != 0 || u.Source != "dynamic_yahoo_screener" {
			t.Fatalf("err=%v screens=%d", err, f.screens)
		}
	})
	t.Run("viejo: se redescubre y se guarda el de hoy", func(t *testing.T) {
		dir := t.TempDir()
		writeSnapshot(t, dir, 4, now)
		f := &fakeYahoo{}
		u, err := resolveUniverse(context.Background(), &jobs.Env{Yahoo: f}, dir, nil, now, quiet, quiet)
		if err != nil || f.screens != 7 || len(u.Asia) != 20 {
			t.Fatalf("err=%v screens=%d asia=%d", err, f.screens, len(u.Asia))
		}
		if _, err := os.Stat(filepath.Join(dir, "universe", "universe_20261009.json")); err != nil {
			t.Fatal("no guardó el snapshot de hoy:", err)
		}
	})
	t.Run("viejo y el screener falla: conserva el snapshot viejo", func(t *testing.T) {
		screenBackoff = 0
		defer func() { screenBackoff = 3 * time.Second }()
		dir := t.TempDir()
		writeSnapshot(t, dir, 10, now)
		f := &fakeYahoo{screenErr: yahoo.ErrEmptyScreen}
		var avisos []string
		warn := func(f string, a ...any) { avisos = append(avisos, fmt.Sprintf(f, a...)) }
		u, err := resolveUniverse(context.Background(), &jobs.Env{Yahoo: f}, dir, nil, now, quiet, warn)
		if err != nil || len(u.Asia) == 0 {
			t.Fatalf("err=%v", err)
		}
		if _, e := os.Stat(filepath.Join(dir, "universe", "universe_20261009.json")); e == nil {
			t.Error("no debe guardar un snapshot nuevo si el descubrimiento falló")
		}
		if !strings.Contains(strings.Join(avisos, " | "), "no se pudo refrescar") {
			t.Errorf("faltó el aviso: %q", avisos)
		}
	})
	t.Run("skip_discovery: nunca descubre", func(t *testing.T) {
		dir := t.TempDir()
		writeSnapshot(t, dir, 30, now)
		f := &fakeYahoo{}
		if _, err := resolveUniverse(context.Background(), &jobs.Env{Yahoo: f}, dir, map[string]any{"skip_discovery": true}, now, quiet, quiet); err != nil || f.screens != 0 {
			t.Fatalf("err=%v screens=%d", err, f.screens)
		}
	})
	t.Run("force_discovery: descubre aunque haya snapshot de hoy", func(t *testing.T) {
		dir := t.TempDir()
		writeSnapshot(t, dir, 0, now)
		f := &fakeYahoo{}
		if _, err := resolveUniverse(context.Background(), &jobs.Env{Yahoo: f}, dir, map[string]any{"force_discovery": true}, now, quiet, quiet); err != nil || f.screens != 7 {
			t.Fatalf("err=%v screens=%d", err, f.screens)
		}
	})
}
