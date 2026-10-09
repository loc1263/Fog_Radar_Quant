package cl_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fogradar/internal/api"
	"fogradar/internal/core"
	"fogradar/internal/modules/cl"
)

// Mismo esquema que SCHEMA_V1 de modules/cl_stocks/history.py (las bases de Python y Go son la misma).
const schemaV1 = `
CREATE TABLE runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT, ts TEXT UNIQUE NOT NULL, run_at TEXT NOT NULL,
    archivo TEXT NOT NULL, n_tickers INTEGER NOT NULL, stored_at TEXT NOT NULL DEFAULT (datetime('now')));
CREATE TABLE ranking (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE, ticker TEXT NOT NULL, rank INTEGER,
    nombre TEXT, sector TEXT, industria TEXT, score_compuesto REAL, score_valor REAL, score_calidad REAL,
    score_crecimiento REAL, score_salud_financiera REAL, score_dividendo REAL, score_momentum REAL,
    metricas_disponibles INTEGER, grupo_duplicado INTEGER, PRIMARY KEY (run_pk, ticker));
CREATE TABLE indicadores (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE, ticker TEXT NOT NULL,
    metrica TEXT NOT NULL, valor REAL, PRIMARY KEY (run_pk, ticker, metrica));`

type fixture struct {
	srv *httptest.Server
	s   *core.Settings
}

func newFixture(t *testing.T, mut func(*core.Settings)) *fixture {
	t.Helper()
	dir := t.TempDir()
	loc, _ := time.LoadLocation("America/Santiago")
	s := &core.Settings{Env: "test", AppName: "Test API", RateLimit: "300/minute", DataDir: dir,
		Timezone: "America/Santiago", Loc: loc, Origins: []string{"http://localhost:5173"}}
	if mut != nil {
		mut(s)
	}

	db, err := core.OpenWrite(filepath.Join(dir, "cl_stocks", core.HistDBName))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := core.Migrar(db, []string{schemaV1}); err != nil {
		t.Fatal(err)
	}
	mustExec := func(q string, a ...any) {
		if _, err := db.Exec(q, a...); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(`INSERT INTO runs (id, ts, run_at, archivo, n_tickers) VALUES
		(1, '20260101_100000', '2026-01-01T10:00:00-03:00', 'ranking_acciones_20260101_100000.csv', 2),
		(2, '20260102_100000', '2026-01-02T10:00:00', 'ranking_acciones_20260102_100000.csv', 3)`)
	mustExec(`INSERT INTO ranking (run_pk, ticker, rank, nombre, score_compuesto, score_valor, metricas_disponibles) VALUES
		(1, 'AAA.SN', 1, 'Alfa', 0.9, 0.5, 40), (1, 'BBB.SN', 2, 'Beta', 0.5, NULL, 38),
		(2, 'BBB.SN', 1, 'Beta', 0.95, 0.6, 40), (2, 'AAA.SN', 2, 'Alfa', 0.85, 0.4, 40), (2, 'CCC.SN', 3, 'Gamma', 0.1, NULL, 20)`)
	mustExec(`INSERT INTO indicadores VALUES (1, 'AAA.SN', 'roe', 0.12), (2, 'AAA.SN', 'roe', 0.15), (2, 'BBB.SN', 'roe', NULL)`)
	db.Close()

	h := api.New(s, slog.New(slog.NewTextHandler(io.Discard, nil)))
	cl.Register(h)
	srv := httptest.NewServer(h.Handler())
	t.Cleanup(srv.Close)
	return &fixture{srv, s}
}

func (f *fixture) get(t *testing.T, path string, hdr ...string) (int, map[string]any, []any, http.Header) {
	t.Helper()
	req, _ := http.NewRequest("GET", f.srv.URL+path, nil)
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var obj map[string]any
	var arr []any
	if json.Unmarshal(raw, &obj) != nil {
		_ = json.Unmarshal(raw, &arr)
	}
	return resp.StatusCode, obj, arr, resp.Header
}

func TestHealthEsPublicoYNoTieneSchemaExtra(t *testing.T) {
	f := newFixture(t, func(s *core.Settings) { s.APIKey = "k" })
	code, body, _, hdr := f.get(t, "/api/health")
	if code != 200 || body["status"] != "ok" || body["app"] != "Test API" || len(body) != 4 {
		t.Fatalf("%d %v", code, body)
	}
	if hdr.Get("Link") != "" || hdr.Get("X-Process-Time-Ms") == "" {
		t.Fatalf("headers: %v", hdr)
	}
}

func TestAuthPorAPIKey(t *testing.T) {
	f := newFixture(t, func(s *core.Settings) { s.APIKey = "clave-correcta" })
	for _, k := range []string{"", "mala"} {
		code, body, _, hdr := f.get(t, "/api/status", core.APIKeyHeader, k)
		if code != 401 || body["code"] != "unauthorized" || body["detail"] != "API key inválida o ausente." ||
			hdr.Get("WWW-Authenticate") != "ApiKey" {
			t.Fatalf("clave %q: %d %v %v", k, code, body, hdr)
		}
	}
	if code, _, _, _ := f.get(t, "/api/status", core.APIKeyHeader, "clave-correcta"); code != 200 {
		t.Fatalf("clave correcta: %d", code)
	}
	if code, _, _, _ := f.get(t, "/api/cl/history/runs"); code != 401 {
		t.Fatalf("el historial es protegido: %d", code)
	}
}

func TestEnvelopeDeErrores(t *testing.T) {
	f := newFixture(t, nil)
	casos := []struct {
		path string
		code int
		cod  string
	}{
		{"/api/no-existe", 404, "not_found"},
		{"/api/jobs/modulo-raro", 404, "unknown_module"},
		{"/api/jobs/etf?limit=0", 422, "validation_error"},
		{"/api/jobs/etf?limit=abc", 422, "validation_error"},
		{"/api/cl/history/ticker/ZZZ.SN", 404, "not_found"},
		{"/api/cl/history/ticker/" + string(make([]byte, 0)) + "XXXXXXXXXXXXXXXX", 422, "validation_error"},
		{"/api/cl/history/ticker/AAA.SN/indicador/inventada", 404, "not_found"},
		{"/api/cl/history/diff?desde=20260101_100000", 422, "validation_error"},
		{"/api/cl/history/diff?desde=mal&hasta=mal", 422, "validation_error"},
		{"/api/cl/history/diff?desde=20250101_000000&hasta=20260102_100000", 404, "not_found"},
	}
	for _, c := range casos {
		code, body, _, _ := f.get(t, c.path)
		if code != c.code || body["code"] != c.cod || body["detail"] == nil || len(body) != 2 {
			t.Errorf("%s: %d %v", c.path, code, body)
		}
	}
	req, _ := http.NewRequest("DELETE", f.srv.URL+"/api/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode != 405 || body["code"] != "method_not_allowed" {
		t.Errorf("405: %d %v", resp.StatusCode, body)
	}
}

func TestRateLimit(t *testing.T) {
	f := newFixture(t, func(s *core.Settings) { s.RateLimit = "2/minute"; s.RateLimitInTests = true })
	for i := 0; i < 2; i++ {
		if code, _, _, _ := f.get(t, "/api/health", "X-Forwarded-For", "1.1.1.1"); code != 200 {
			t.Fatalf("hit %d: %d", i, code)
		}
	}
	code, body, _, _ := f.get(t, "/api/health", "X-Forwarded-For", "1.1.1.1")
	if code != 429 || body["code"] != "rate_limited" {
		t.Fatalf("%d %v", code, body)
	}
	if code, _, _, _ := f.get(t, "/api/health", "X-Forwarded-For", "2.2.2.2"); code != 200 {
		t.Fatalf("otra IP no comparte el límite: %d", code)
	}
}

func TestCORS(t *testing.T) {
	f := newFixture(t, nil)
	_, _, _, hdr := f.get(t, "/api/health", "Origin", "http://localhost:5173")
	if hdr.Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("origen permitido: %v", hdr)
	}
	_, _, _, hdr = f.get(t, "/api/health", "Origin", "http://malo.example")
	if hdr.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("origen no permitido: %v", hdr)
	}
	req, _ := http.NewRequest("OPTIONS", f.srv.URL+"/api/status", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Access-Control-Allow-Headers") != "X-API-Key, Content-Type" {
		t.Fatalf("preflight: %d %v", resp.StatusCode, resp.Header)
	}
}

func TestHistorialRuns(t *testing.T) {
	f := newFixture(t, nil)
	code, _, arr, _ := f.get(t, "/api/cl/history/runs")
	if code != 200 || len(arr) != 2 {
		t.Fatal(code, arr)
	}
	r0, r1 := arr[0].(map[string]any), arr[1].(map[string]any)
	if r0["ts"] != "20260101_100000" || r0["top1_ticker"] != "AAA.SN" || r0["top1_score"] != 0.9 {
		t.Fatalf("%v", r0)
	}
	if r0["run_at"] != "2026-01-01T10:00:00-03:00" {
		t.Fatalf("run_at con offset: %v", r0["run_at"])
	}
	if r1["run_at"] != "2026-01-02T10:00:00-03:00" { // sin zona en la base → Settings.timezone (enero: -03:00)
		t.Fatalf("run_at sin zona: %v", r1["run_at"])
	}
}

func TestHistorialSinBaseDevuelveListaVacia(t *testing.T) {
	f := newFixture(t, nil)
	_ = os.RemoveAll(filepath.Join(f.s.DataDir, "cl_stocks"))
	code, _, arr, _ := f.get(t, "/api/cl/history/runs")
	if code != 200 || arr == nil || len(arr) != 0 {
		t.Fatalf("debe ser [] y no null: %d %v", code, arr)
	}
}

func TestHistorialTickerEIndicador(t *testing.T) {
	f := newFixture(t, nil)
	code, _, arr, _ := f.get(t, "/api/cl/history/ticker/aaa.sn")
	if code != 200 || len(arr) != 2 {
		t.Fatal(code, arr)
	}
	p := arr[0].(map[string]any)
	pil := p["pilares"].(map[string]any)
	if p["rank"] != 1.0 || p["nombre"] != "Alfa" || pil["valor"] != 0.5 || pil["calidad"] != nil || len(pil) != 6 {
		t.Fatalf("%v", p)
	}
	_, _, arr, _ = f.get(t, "/api/cl/history/ticker/AAA.SN/indicador/roe")
	if len(arr) != 2 || arr[1].(map[string]any)["valor"] != 0.15 {
		t.Fatalf("%v", arr)
	}
	_, _, arr, _ = f.get(t, "/api/cl/history/ticker/BBB.SN/indicador/roe")
	if len(arr) != 1 || arr[0].(map[string]any)["valor"] != nil {
		t.Fatalf("el NULL de la base sale como null: %v", arr)
	}
}

func TestHistorialDiff(t *testing.T) {
	f := newFixture(t, nil)
	code, body, _, _ := f.get(t, "/api/cl/history/diff")
	if code != 200 || body["desde"] != "20260101_100000" || body["hasta"] != "20260102_100000" || body["top_n"] != nil {
		t.Fatal(code, body)
	}
	filas := body["filas"].([]any)
	got := ""
	for _, x := range filas {
		fm := x.(map[string]any)
		got += fm["ticker"].(string) + ":" + fm["estado"].(string) + " "
	}
	if got != "BBB.SN:sigue AAA.SN:sigue CCC.SN:nuevo " {
		t.Fatal(got)
	}
	if d := filas[0].(map[string]any)["delta_rank"]; d != 1.0 {
		t.Fatalf("BBB subió 1 puesto: %v", d)
	}
}

func TestStatusYJobs(t *testing.T) {
	f := newFixture(t, nil)
	code, body, _, _ := f.get(t, "/api/status")
	mods := body["modules"].([]any)
	if code != 200 || len(mods) != 4 {
		t.Fatal(code, body)
	}
	m0 := mods[0].(map[string]any)
	if m0["key"] != "cl_stocks" || m0["api_prefix"] != "/api/cl" || m0["data_available"] != true ||
		m0["latest_job"] != nil || m0["migration_phase"] != 3.0 {
		t.Fatalf("%v", m0)
	}
	if mods[1].(map[string]any)["data_available"] != false {
		t.Fatal("us_stocks no tiene carpeta de datos")
	}
	code, body, _, _ = f.get(t, "/api/jobs")
	if code != 200 || len(body) != 4 || body["etf"] != nil {
		t.Fatalf("%d %v", code, body)
	}
	_, _, arr, _ := f.get(t, "/api/jobs/programas")
	if len(arr) != 2 || arr[1].(map[string]any)["cron"] != "0 18 * * 1-5" {
		t.Fatalf("%v", arr)
	}
	_, _, arr, _ = f.get(t, "/api/jobs/etf")
	if arr == nil || len(arr) != 0 {
		t.Fatalf("sin registro → []: %v", arr)
	}
	code, body, _, _ = f.get(t, "/api/cl/status")
	if code != 200 || body["key"] != "cl_stocks" {
		t.Fatal(code, body)
	}
}

func TestOpenAPIEnDevYNoEnProd(t *testing.T) {
	f := newFixture(t, nil)
	code, body, _, _ := f.get(t, "/openapi.json")
	if code != 200 || body["openapi"] == nil {
		t.Fatalf("dev sirve el OpenAPI: %d", code)
	}
	paths := body["paths"].(map[string]any)
	for _, p := range []string{"/api/health", "/api/status", "/api/cl/history/diff", "/api/jobs/{module}"} {
		if paths[p] == nil {
			t.Errorf("falta %s en el OpenAPI", p)
		}
	}
	prod := newFixture(t, func(s *core.Settings) { s.Env = "prod"; s.APIKey = "k" })
	for _, p := range []string{"/openapi.json", "/docs"} {
		if code, _, _, _ := prod.get(t, p); code != 404 {
			t.Errorf("en prod %s debe dar 404, dio %d", p, code)
		}
	}
}
