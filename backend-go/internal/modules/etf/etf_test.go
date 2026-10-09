package etf

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"fogradar/internal/quant"
)

// copiarFixtures copia (descomprimiendo los .gz) los archivos de testdata/<sub> a destino.
func copiarFixtures(t *testing.T, sub, destino string) {
	t.Helper()
	if err := os.MkdirAll(destino, 0o755); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(filepath.Join("testdata", sub))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		b := leer(t, filepath.Join("testdata", sub, e.Name()))
		name := strings.TrimSuffix(e.Name(), ".gz")
		if err := os.WriteFile(filepath.Join(destino, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func leer(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func esperado(t *testing.T, nombre string) []byte {
	t.Helper()
	p := filepath.Join("testdata", "expected", nombre)
	if _, err := os.Stat(p); err != nil {
		p += ".gz"
	}
	return leer(t, p)
}

func norm(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

var salidas = []string{
	Top15File, RankingFile, ExcluidosFile, Top15SimpleFile, MomentumRanking, MomentumTop15,
	CategoriasFile, CapPortfolio, CapBuckets, HistorialFile,
}

func analizado(t *testing.T) (*Analisis, string) {
	t.Helper()
	dir := t.TempDir()
	data, hist := filepath.Join(dir, "current"), filepath.Join(dir, "hist")
	copiarFixtures(t, "input", data)
	copiarFixtures(t, "hist", hist)
	a, err := CorrerAnalisis(data, hist, t.Logf)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Avisos) != 0 {
		t.Fatalf("avisos: %v", a.Avisos)
	}
	return a, data
}

// Paridad con ETF_Choice: mismo etf_perf_data + mismo histórico → mismos archivos, byte a byte.
func TestSalidasByteAByte(t *testing.T) {
	_, data := analizado(t)
	for _, n := range salidas {
		got, err := os.ReadFile(filepath.Join(data, n))
		if err != nil {
			t.Errorf("%s: %v", n, err)
			continue
		}
		want := esperado(t, n)
		if n == CategoriasFile {
			// Excepción documentada: el orden de los tickers EMPATADOS en Sharpe dentro de una categoría lo decide
			// el argsort de numpy (no estable y dependiente de la CPU: SIMD AVX-512); Go usa orden estable.
			got, want = ordenarTickers(got), ordenarTickers(want)
		}
		if !bytes.Equal(norm(got), norm(want)) {
			gl, wl := strings.Split(string(norm(got)), "\n"), strings.Split(string(norm(want)), "\n")
			if len(gl) != len(wl) {
				t.Errorf("%s: %d líneas (go) vs %d (original)", n, len(gl), len(wl))
			}
			for i := 0; i < len(wl) && i < len(gl); i++ {
				if gl[i] != wl[i] {
					t.Errorf("%s: difiere en la línea %d\n go: %.300s\norig: %.300s", n, i+1, gl[i], wl[i])
					break
				}
			}
		}
	}
}

func TestResumenDeLaCorrida(t *testing.T) {
	a, _ := analizado(t)
	if a.FechaTag != "20260924" || a.NUniversoBruto != 1189 || a.NUniversoDepurad != 683 || a.NOutliers != 8 {
		t.Fatalf("%s %d %d %d", a.FechaTag, a.NUniversoBruto, a.NUniversoDepurad, a.NOutliers)
	}
	if a.Top15.Col("Ticker").Cell(0) != "CRAK" {
		t.Fatal(a.Top15.Col("Ticker").Cell(0))
	}
}

// ordenarTickers ordena alfabéticamente la lista de tickers (última columna antes de Fecha) de cada fila.
func ordenarTickers(b []byte) []byte {
	lines := strings.Split(string(norm(b)), "\n")
	for i, l := range lines {
		a, c := strings.Index(l, `"`), strings.LastIndex(l, `"`)
		if a < 0 || c <= a {
			continue
		}
		toks := strings.Split(l[a+1:c], ", ")
		sort.Strings(toks)
		lines[i] = l[:a+1] + strings.Join(toks, ", ") + l[c:]
	}
	return []byte(strings.Join(lines, "\n"))
}

func cargarJSON(t *testing.T, nombre string, v any) {
	t.Helper()
	if err := json.Unmarshal(esperado(t, nombre), v); err != nil {
		t.Fatal(err)
	}
}

func TestRotacionIgualAlOriginal(t *testing.T) {
	dir := t.TempDir()
	data, hist := filepath.Join(dir, "current"), filepath.Join(dir, "hist")
	copiarFixtures(t, "input", data)
	copiarFixtures(t, "hist", hist)
	for _, n := range salidas {
		if err := os.WriteFile(filepath.Join(data, n), esperado(t, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rd := func(n string) *quant.Frame {
		f, err := quant.ReadCSVFile(filepath.Join(data, n))
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	top15, ranking, excl := rd(Top15File), rd(RankingFile), rd(ExcluidosFile)
	rot, err := GenerarDatosRotacion(data, hist, top15, excl, ranking)
	if err != nil || rot == nil {
		t.Fatal(rot, err)
	}
	var exp struct {
		Mantienen, Nuevos, Salidas []map[string]any
		Diversificacion            []map[string]any
	}
	cargarJSON(t, "rotacion.json", &exp)
	nf := func(p *float64) any {
		if p == nil {
			return nil
		}
		return *p
	}
	ni := func(p *int64) any {
		if p == nil {
			return nil
		}
		return float64(*p)
	}
	if len(rot.Mantienen) != len(exp.Mantienen) || len(rot.Nuevos) != len(exp.Nuevos) || len(rot.Salidas) != len(exp.Salidas) {
		t.Fatalf("largos %d/%d/%d vs %d/%d/%d", len(rot.Mantienen), len(rot.Nuevos), len(rot.Salidas), len(exp.Mantienen), len(exp.Nuevos), len(exp.Salidas))
	}
	// to_json de pandas redondea a 10 decimales: se compara con tolerancia relativa 1e-9
	chk := func(what string, got, want any) {
		t.Helper()
		var g, w any
		gj, _ := json.Marshal(got)
		wj, _ := json.Marshal(want)
		_ = json.Unmarshal(gj, &g)
		_ = json.Unmarshal(wj, &w)
		if !casiIguales(g, w) {
			t.Errorf("%s: %s (go) vs %s (py)", what, gj, wj)
		}
	}
	for i, r := range rot.Mantienen {
		e := exp.Mantienen[i]
		w := fmt.Sprintf("mantienen[%d] %s", i, r.Ticker)
		chk(w+" ticker", r.Ticker, e["Ticker"])
		chk(w+" racha", ni(r.Racha), e["Racha"])
		chk(w+" rank", []any{ni(r.RankActual), ni(r.RankPrevio), ni(r.DeltaRank)}, []any{e["Rank_actual"], e["Rank_previo"], e["Delta_Rank"]})
		chk(w+" score", []any{nf(r.Score), nf(r.DeltaScore), nf(r.Precio), nf(r.High52), nf(r.Low52)}, []any{e["Score"], e["Delta_Score"], e["Precio"], e["52W High"], e["52W Low"]})
	}
	for i, r := range rot.Nuevos {
		e := exp.Nuevos[i]
		chk("nuevos ticker", r.Ticker, e["Ticker"])
		chk("nuevos "+r.Ticker, []any{*r.Procedencia, ni(r.RankActual), nf(r.Score)}, []any{e["Procedencia"], e["Rank_actual"], e["Score"]})
	}
	for i, r := range rot.Salidas {
		e := exp.Salidas[i]
		chk("salidas ticker", r.Ticker, e["Ticker"])
		chk("salidas "+r.Ticker, []any{*r.Destino, ni(r.RankPrevio), nf(r.ScorePrevio)}, []any{e["Destino"], e["Rank_previo"], e["Score_previo"]})
	}
	div := DiversificacionTop15(top15)
	if len(div) != len(exp.Diversificacion) {
		t.Fatal(len(div), len(exp.Diversificacion))
	}
	// Mismo conteo por categoría; el orden entre categorías empatadas lo decide el sort no estable de pandas.
	esp := map[string]float64{}
	for _, e := range exp.Diversificacion {
		esp[e["Category"].(string)] = e["Conteo"].(float64)
	}
	for i, d := range div {
		if esp[d.Category] != float64(d.Conteo) {
			t.Errorf("diversificacion[%d] %s: %d vs %v", i, d.Category, d.Conteo, esp[d.Category])
		}
		if i > 0 && div[i-1].Conteo < d.Conteo {
			t.Errorf("no está ordenado por conteo descendente")
		}
	}
	if p := EncontrarArchivoPrevio(hist, Top15File); filepath.Base(p) != "etf_top15_recomendados_20260922.csv" {
		t.Fatal(p)
	}
}

func TestArchivarCorridaConSecuencia(t *testing.T) {
	dir := t.TempDir()
	data, hist := filepath.Join(dir, "current"), filepath.Join(dir, "hist")
	for i := 0; i < 3; i++ {
		copiarFixtures(t, "input", data)
		for _, n := range OutputFilesDateless {
			if err := os.WriteFile(filepath.Join(data, n), []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		mov, err := ArchivarCorrida(data, hist)
		if err != nil || len(mov) != 9 {
			t.Fatal(len(mov), err)
		}
	}
	for _, n := range []string{"etf_perf_data_20260924.csv", "etf_perf_data_20260924_2.csv", "etf_perf_data_20260924_3.csv", "etf_top15_recomendados_20260924_3.csv"} {
		if !fileOK(filepath.Join(hist, n)) {
			t.Errorf("falta %s", n)
		}
	}
	if m, _ := filepath.Glob(filepath.Join(data, "etf_*")); len(m) != 0 {
		t.Fatalf("quedó algo en current: %v", m)
	}
	if f, s := FechaYSecuencia("etf_ranking_completo_20260917_2.csv"); f != "20260917" || s != 2 {
		t.Fatal(f, s)
	}
	if f, s := FechaYSecuencia("raro.csv"); f != "00000000" || s != 1 {
		t.Fatal(f, s)
	}
}

func casiIguales(a, b any) bool {
	switch x := a.(type) {
	case float64:
		y, ok := b.(float64)
		return ok && math.Abs(x-y) <= 1e-9*math.Max(1, math.Abs(y))
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !casiIguales(x[i], y[i]) {
				return false
			}
		}
		return true
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func TestInformePDF(t *testing.T) {
	dir := t.TempDir()
	data, hist := filepath.Join(dir, "current"), filepath.Join(dir, "hist")
	copiarFixtures(t, "input", data)
	copiarFixtures(t, "hist", hist)
	for _, n := range salidas {
		if err := os.WriteFile(filepath.Join(data, n), esperado(t, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, fecha, err := GenerarPDF(data, hist)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) || len(b) < 10_000 {
		t.Fatalf("PDF inválido (%d bytes)", len(b))
	}
	if len(fecha) != 10 || fecha[2] != '/' {
		t.Errorf("fecha de corrida %q", fecha)
	}
	if out := os.Getenv("ETF_PDF_OUT"); out != "" {
		if err := os.WriteFile(out, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := GenerarPDF(t.TempDir(), hist); !errors.Is(err, ErrSinCorrida) {
		t.Errorf("sin corrida: %v", err)
	}
}
