package main

import (
	"strings"
	"testing"

	"fogradar/internal/jobs"
)

func TestParseNoCache(t *testing.T) {
	a, err := parseJobArgs([]string{"--todos", "--no-cache"})
	if err != nil {
		t.Fatal(err)
	}
	if !a.noCache || !a.todos {
		t.Errorf("noCache=%v todos=%v", a.noCache, a.todos)
	}
	if a, err := parseJobArgs([]string{"--todos"}); err != nil || a.noCache {
		t.Errorf("sin la bandera no debe activarse: %v %v", a, err)
	}
}

func TestNoCacheYOfflineSonExcluyentes(t *testing.T) {
	_, err := parseJobArgs([]string{"--todos", "--no-cache", "--offline"})
	if err == nil || !strings.Contains(err.Error(), "excluyentes") {
		t.Fatalf("esperaba error de exclusión, fue %v", err)
	}
}

func TestAplicarSinCache(t *testing.T) {
	mods := []string{"cl_stocks", "us_stocks", "tendencia", "etf"}
	opciones := map[string]map[string]any{}
	aplicarSinCache(opciones, mods)

	if got := opciones["etf"]; got["refrescar_info"] != true {
		t.Errorf("etf: %v", got)
	}
	if got := opciones["tendencia"]; got["force_refresh"] != true || got["force_discovery"] != true {
		t.Errorf("tendencia: %v", got)
	}
	for _, m := range []string{"cl_stocks", "us_stocks"} { // sin caché: no se les agrega nada
		if _, ok := opciones[m]; ok {
			t.Errorf("%s no tiene caché y no debía recibir opciones: %v", m, opciones[m])
		}
	}
}

func TestAplicarSinCacheNoPisaLoDelUsuario(t *testing.T) {
	opciones := map[string]map[string]any{"tendencia": {"force_refresh": false, "top_n": 20}}
	aplicarSinCache(opciones, []string{"tendencia"})
	got := opciones["tendencia"]
	if got["force_refresh"] != false || got["top_n"] != 20 || got["force_discovery"] != true {
		t.Errorf("lo puesto a mano debe ganar y lo demás completarse: %v", got)
	}
}

func TestOpcionesSinCacheSonDeclaradasPorSusModulos(t *testing.T) {
	// Cada opción de OpcionesSinCache debe existir como bandera del módulo (si alguien la renombra, esto avisa).
	for mod, opts := range jobs.OpcionesSinCache {
		p, ok := jobs.Get(mod)
		if !ok {
			continue // los pipelines se registran al importar el paquete del módulo (ver main.go)
		}
		declaradas := map[string]bool{}
		for _, o := range p.Options {
			declaradas[o.Dest()] = true
		}
		for k := range opts {
			if !declaradas[k] {
				t.Errorf("%s: %q no es una opción declarada del módulo", mod, k)
			}
		}
	}
}
