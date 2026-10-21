package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/api"
	"fogradar/internal/core"
)

type vacio struct{}

type listaOut struct {
	Body []map[string]string
}

type unoOut struct {
	Body map[string]string
}

func hostDePrueba(t *testing.T) *api.Host {
	t.Helper()
	s := &core.Settings{Env: "test", DataDir: t.TempDir(), Loc: time.UTC, RateLimit: "1000/minute"}
	h := api.New(s, slog.New(slog.DiscardHandler))
	op := func(id, path string) huma.Operation {
		return huma.Operation{OperationID: id, Method: http.MethodGet, Path: path}
	}
	api.Public(h, op("a", "/api/etf/history/top"), func(context.Context, *vacio) (*listaOut, error) {
		return &listaOut{Body: []map[string]string{{"ticker": "SPY"}, {"ticker": "QQQ"}, {"ticker": "SPY"}}}, nil
	})
	api.Public(h, op("b", "/api/etf/history/ticker/{ticker}"), func(_ context.Context, in *struct {
		Ticker string `path:"ticker"`
	}) (*unoOut, error) {
		return &unoOut{Body: map[string]string{"ticker": in.Ticker}}, nil
	})
	api.Public(h, op("c", "/api/cl/ranking"), func(context.Context, *vacio) (*unoOut, error) {
		return nil, core.Err(http.StatusNotFound, "not_found", "sin datos")
	})
	return h
}

func TestExportarEscribeJSONYExpande(t *testing.T) {
	h := hostDePrueba(t)
	out := t.TempDir()
	m, err := exportar(h, &exportArgs{out: out}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"api/etf/history/top.json", "api/etf/history/ticker/SPY.json", "api/etf/history/ticker/QQQ.json", "manifest.json"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(r))); err != nil {
			t.Errorf("falta %s: %v", r, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "api", "cl", "ranking.json")); err == nil {
		t.Error("una respuesta 404 no debe escribir archivo")
	}
	estados := map[string]int{}
	for _, e := range m.Archivos {
		estados[e.Ruta] = e.Estado
	}
	if estados["/api/cl/ranking"] != http.StatusNotFound || len(estados) < 4 {
		t.Errorf("manifest: %v", estados)
	}
	var body map[string]string
	b, _ := os.ReadFile(filepath.Join(out, "api", "etf", "history", "ticker", "SPY.json"))
	if json.Unmarshal(b, &body) != nil || body["ticker"] != "SPY" {
		t.Errorf("contenido: %s", b)
	}
}

func TestSinFuera(t *testing.T) {
	in := `{"desde":"a","hasta":"b","filas":[{"ticker":"X","estado":"nuevo"},{"ticker":"Y","estado":"fuera"}]}`
	var d struct {
		Desde string `json:"desde"`
		Filas []struct {
			Ticker string `json:"ticker"`
		} `json:"filas"`
	}
	if err := json.Unmarshal(sinFuera([]byte(in)), &d); err != nil {
		t.Fatal(err)
	}
	if d.Desde != "a" || len(d.Filas) != 1 || d.Filas[0].Ticker != "X" {
		t.Errorf("%+v", d)
	}
}

func TestExportarFiltraPorModulo(t *testing.T) {
	h := hostDePrueba(t)
	m, err := exportar(h, &exportArgs{out: t.TempDir(), modulos: []string{"etf"}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range m.Archivos {
		if e.Ruta == "/api/cl/ranking" {
			t.Error("cl no debía exportarse")
		}
	}
}

func TestParseExportArgs(t *testing.T) {
	a, err := parseExportArgs([]string{"--out", "x", "--modulo", "etf, cl"})
	if err != nil || a.out != "x" || len(a.modulos) != 2 || a.modulos[1] != "cl" {
		t.Fatalf("%+v %v", a, err)
	}
}

func TestFilasDe(t *testing.T) {
	if got := filasDe([]byte(`[{"a":"1"}]`), ""); len(got) != 1 {
		t.Errorf("arreglo raíz: %v", got)
	}
	if got := filasDe([]byte(`{"filas":[{"a":"1"},{"a":"2"}]}`), "filas"); len(got) != 2 {
		t.Errorf("clave filas: %v", got)
	}
}

func TestDiffsDeCorridas(t *testing.T) {
	defer func(orig []paresDeDiff) { diffsDeCorridas = orig }(diffsDeCorridas)
	s := &core.Settings{Env: "test", DataDir: t.TempDir(), Loc: time.UTC, RateLimit: "1000/minute"}
	h := api.New(s, slog.New(slog.DiscardHandler))
	api.Public(h, huma.Operation{OperationID: "r", Method: http.MethodGet, Path: "/api/zz/runs"},
		func(context.Context, *vacio) (*listaOut, error) {
			return &listaOut{Body: []map[string]string{{"k": "1"}, {"k": "2"}, {"k": "3"}}}, nil
		})
	api.Public(h, huma.Operation{OperationID: "d", Method: http.MethodGet, Path: "/api/zz/diff"},
		func(_ context.Context, in *struct {
			Desde string `query:"desde"`
			Hasta string `query:"hasta"`
		}) (*unoOut, error) {
			return &unoOut{Body: map[string]string{"desde": in.Desde, "hasta": in.Hasta}}, nil
		})
	for todos, esperados := range map[bool]int{false: 2, true: 3} {
		diffsDeCorridas = []paresDeDiff{{"/api/zz/runs", "k", "/api/zz/diff", todos}}
		out := t.TempDir()
		m, err := exportar(h, &exportArgs{out: out, modulos: []string{"zz"}}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range m.Archivos {
			if len(e.Ruta) > 13 && e.Ruta[:13] == "/api/zz/diff?" {
				n++
			}
		}
		if n != esperados {
			t.Errorf("todos=%v: %d diffs, esperaba %d", todos, n, esperados)
		}
	}
}
