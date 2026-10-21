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
