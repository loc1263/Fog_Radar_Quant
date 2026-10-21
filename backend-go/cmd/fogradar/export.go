package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fogradar/internal/api"
	"fogradar/internal/core"
)

// Instantánea estática de la API (sitio en GitHub Pages): llama en proceso a cada GET sin parámetros
// obligatorios y guarda la respuesta en <out>/<ruta>.json (los PDF, en .pdf). Sin red ni rate limit.

// expansion pide, además, un GET por cada valor de `campo` de la respuesta de `origen`.
type expansion struct {
	origen string // ruta ya exportada, p. ej. /api/etf/history/top
	lista  string // clave del objeto que contiene el arreglo ("" si la respuesta es el arreglo)
	campo  string // campo de cada elemento, p. ej. ticker
	plan   string // ruta con un parámetro, p. ej. /api/etf/history/ticker/{ticker}
	param  string // nombre del parámetro de `plan`
}

var expansiones = []expansion{
	{"/api/etf/history/top", "", "ticker", "/api/etf/history/ticker/{ticker}", "ticker"},
	{"/api/cl/ranking", "filas", "simbolo", "/api/cl/history/ticker/{ticker}", "ticker"},
}

// paresDeDiff exporta diffs entre corridas (query desde, hasta) como <diff>/<desde>__<hasta>.json.
// Con todos=false solo cada corrida contra la anterior (universos grandes: N² pares no caben en un
// sitio estático); con todos=true, cada par desde<hasta. Se descartan las filas «fuera» (fuera del
// top en ambas corridas): son todo el universo y no aportan al diff.
type paresDeDiff struct {
	runs  string // ruta ya exportada con la clave de cada corrida, en orden cronológico
	clave string // campo de la corrida, p. ej. run_key
	diff  string // ruta de diff
	todos bool
}

var diffsDeCorridas = []paresDeDiff{
	{"/api/etf/history/runs", "run_key", "/api/etf/history/diff", false},
	{"/api/cl/history/runs", "ts", "/api/cl/history/diff", true},
}

type exportArgs struct {
	out     string
	modulos []string // vacío = todos
}

type exportEntrada struct {
	Ruta   string `json:"ruta"`
	Estado int    `json:"estado"`
	Bytes  int    `json:"bytes,omitempty"`
}

type exportManifest struct {
	GeneradoEn string          `json:"generado_en"`
	Version    string          `json:"version"`
	Archivos   []exportEntrada `json:"archivos"`
}

func parseExportArgs(args []string) (*exportArgs, error) {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	out := fs.String("out", filepath.Join("..", "frontend", "static", "data"), "carpeta de salida")
	mods := fs.String("modulo", "", "módulos a exportar, separados por coma (etf,cl,us,tendencia); vacío = todos")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	a := &exportArgs{out: *out}
	for _, m := range strings.Split(*mods, ",") {
		if m = strings.TrimSpace(m); m != "" {
			a.modulos = append(a.modulos, m)
		}
	}
	return a, nil
}

// rutasExportables lista los GET sin parámetros obligatorios (ni de ruta ni de query), ordenados.
func rutasExportables(h *api.Host, modulos []string) []string {
	var rutas []string
	for path, item := range h.API.OpenAPI().Paths {
		if item.Get == nil || !strings.HasPrefix(path, "/api/") || path == "/api/health" {
			continue
		}
		requerido := false
		for _, p := range item.Get.Parameters {
			requerido = requerido || p.Required || p.In == "path"
		}
		if requerido || !moduloIncluido(path, modulos) {
			continue
		}
		rutas = append(rutas, path)
	}
	sort.Strings(rutas)
	return rutas
}

func moduloIncluido(path string, modulos []string) bool {
	if len(modulos) == 0 {
		return true
	}
	for _, m := range modulos {
		if strings.HasPrefix(path, "/api/"+m+"/") {
			return true
		}
	}
	return false
}

// archivoDe convierte /api/etf/top15 en <out>/api/etf/top15.json.
func archivoDe(out, ruta, ext string) string {
	return filepath.Join(out, filepath.FromSlash(strings.TrimPrefix(ruta, "/")+ext))
}

func exportar(h *api.Host, a *exportArgs, now time.Time) (*exportManifest, error) {
	handler := h.Handler()
	m := &exportManifest{GeneradoEn: now.In(h.S.Loc).Format(time.RFC3339), Version: core.Version}
	cuerpos := map[string][]byte{}
	var pedirComo func(ruta, archivo string, filtro func([]byte) []byte)

	pedir := func(ruta string) { pedirComo(ruta, ruta, nil) }
	pedirComo = func(ruta, archivo string, filtro func([]byte) []byte) {
		req := httptest.NewRequest(http.MethodGet, ruta, nil)
		req.Header.Set(core.APIKeyHeader, h.S.APIKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		e := exportEntrada{Ruta: ruta, Estado: rec.Code}
		tipo := rec.Header().Get("Content-Type")
		ext := map[bool]string{true: ".json", false: ".pdf"}[strings.Contains(tipo, "json")]
		if rec.Code == http.StatusOK && (ext == ".json" || strings.Contains(tipo, "pdf")) {
			body := rec.Body.Bytes()
			if filtro != nil {
				body = filtro(body)
			}
			cuerpos[archivo] = body
			e.Bytes = len(body)
			if err := escribir(archivoDe(a.out, archivo, ext), body); err != nil {
				e.Estado = http.StatusInternalServerError
			}
		}
		m.Archivos = append(m.Archivos, e)
	}

	for _, r := range rutasExportables(h, a.modulos) {
		pedir(r)
	}
	for _, x := range expansiones {
		if !moduloIncluido(x.plan, a.modulos) {
			continue
		}
		filas := filasDe(cuerpos[x.origen], x.lista)
		vistos := map[string]bool{}
		for _, f := range filas {
			v, _ := f[x.campo].(string)
			if v == "" || vistos[v] {
				continue
			}
			vistos[v] = true
			pedir(strings.Replace(x.plan, "{"+x.param+"}", v, 1))
		}
	}

	for _, p := range diffsDeCorridas {
		if !moduloIncluido(p.diff, a.modulos) {
			continue
		}
		var runs []map[string]any
		if json.Unmarshal(cuerpos[p.runs], &runs) != nil {
			continue
		}
		for j := 1; j < len(runs); j++ {
			hasta, _ := runs[j][p.clave].(string)
			desde := 0
			if !p.todos {
				desde = j - 1
			}
			for i := desde; i < j; i++ {
				d, _ := runs[i][p.clave].(string)
				if d != "" && hasta != "" {
					pedirComo(p.diff+"?desde="+d+"&hasta="+hasta, p.diff+"/"+d+"__"+hasta, sinFuera)
				}
			}
		}
	}

	if err := os.MkdirAll(a.out, 0o755); err != nil {
		return nil, err
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	return m, escribir(filepath.Join(a.out, "manifest.json"), b)
}

// filasDe devuelve los elementos de la respuesta (o de su clave `lista`).
func filasDe(body []byte, lista string) []map[string]any {
	var filas []map[string]any
	if lista == "" {
		_ = json.Unmarshal(body, &filas)
		return filas
	}
	var obj map[string][]map[string]any
	_ = json.Unmarshal(body, &obj)
	return obj[lista]
}

// sinFuera quita de un diff las filas con estado «fuera».
func sinFuera(body []byte) []byte {
	var d map[string]any
	if json.Unmarshal(body, &d) != nil {
		return body
	}
	filas, _ := d["filas"].([]any)
	utiles := make([]any, 0, len(filas))
	for _, f := range filas {
		if m, ok := f.(map[string]any); ok && m["estado"] == "fuera" {
			continue
		}
		utiles = append(utiles, f)
	}
	d["filas"] = utiles
	out, err := json.Marshal(d)
	if err != nil {
		return body
	}
	return out
}

func escribir(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func runExport(args []string, stdout, stderr io.Writer) int {
	a, err := parseExportArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "fogradar export:", err)
		return exitUso
	}
	// Llamadas en proceso: el límite por IP no aplica. Se lee al registrar las rutas, así que va antes de newHost.
	_ = os.Setenv("RATE_LIMIT_DEFAULT", "1000000/minute")
	h, err := newHost()
	if err != nil {
		fmt.Fprintln(stderr, "fogradar export:", err)
		return exitFalla
	}
	m, err := exportar(h, a, time.Now())
	if err != nil {
		fmt.Fprintln(stderr, "fogradar export:", err)
		return exitFalla
	}
	ok, sin, mal := 0, 0, 0
	for _, e := range m.Archivos {
		switch {
		case e.Estado == http.StatusOK:
			ok++
		case e.Estado >= 500:
			mal++
			fmt.Fprintf(stderr, "  %d %s\n", e.Estado, e.Ruta)
		default:
			sin++ // 404/409: el módulo aún no tiene datos
			fmt.Fprintf(stdout, "  %d %s (omitido)\n", e.Estado, e.Ruta)
		}
	}
	fmt.Fprintf(stdout, "export: %d archivos en %s (%d omitidos, %d con error)\n", ok, a.out, sin, mal)
	if mal > 0 {
		return exitFalla
	}
	return exitOK
}
