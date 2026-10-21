package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func logConsola(lvl slog.Level) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(newConsoleHandler(&buf, lvl)), &buf
}

func lineas(b *bytes.Buffer) []string {
	return strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
}

func TestConsolaFormatoLegible(t *testing.T) {
	log, buf := logConsola(slog.LevelInfo)
	log = log.With("proc", "job", "pid", 7)
	log.Info("proceso", "modulo", "etf", "opciones", "map[a:b]")
	log.Warn("sin histórico", "ticker", "ABC")
	log.Error("falló", "error", "boom")
	out := buf.String()
	for _, no := range []string{"time=", "level=", "proc=", "pid=", "msg="} {
		if strings.Contains(out, no) {
			t.Errorf("la consola no debe mostrar %q:\n%s", no, out)
		}
	}
	ls := lineas(buf)
	if len(ls) != 3 {
		t.Fatalf("%d líneas:\n%s", len(ls), out)
	}
	for i, want := range []string{"INFO   proceso  modulo=etf  opciones=map[a:b]", "AVISO  sin histórico  ticker=ABC", "ERROR  falló  error=boom"} {
		if !strings.Contains(ls[i], want) {
			t.Errorf("línea %d = %q, quería que contenga %q", i, ls[i], want)
		}
		if len(ls[i]) < 8 || ls[i][2] != ':' || ls[i][5] != ':' { // HH:MM:SS al inicio
			t.Errorf("línea %d sin hora: %q", i, ls[i])
		}
	}
}

func TestConsolaAvanceEnVezDeUnaLineaPorTicker(t *testing.T) {
	log, buf := logConsola(slog.LevelInfo)
	const total = 1226
	for i := 1; i <= total; i++ {
		msg := "ETF"
		if i%3 == 0 { // los inválidos cuentan en la misma barra
			msg = "ETF inválido"
		}
		log.Info(msg, "n", fmt.Sprintf("%d/%d | %5.1f%%", i, total, float64(i)/total*100), "ticker", "X")
	}
	ls := lineas(buf)
	if len(ls) > 12 {
		t.Fatalf("%d líneas de avance, quería ≤ 12 (una por décimo):\n%s", len(ls), buf.String())
	}
	if !strings.Contains(ls[0], "1/1226") || !strings.Contains(ls[len(ls)-1], "1226/1226") || !strings.Contains(ls[len(ls)-1], "100%") {
		t.Errorf("la barra debe empezar en 1 y terminar en 100 %%:\n%s", buf.String())
	}
	if !strings.Contains(ls[len(ls)-1], "[####################]") {
		t.Errorf("barra completa: %q", ls[len(ls)-1])
	}
}

func TestConsolaAvanceDeSectoresYSilencios(t *testing.T) {
	log, buf := logConsola(slog.LevelInfo)
	for i := 1; i <= 20; i++ {
		log.Info(fmt.Sprintf("[Asia] [sector] (%d/20) 005930.KS: Technology", i))
		log.Info("archivado: etf_ranking_completo_20261020_2.csv")
		log.Info("evaluado", "ticker", "MU", "score", 72.3)
	}
	log.Info("instalados en current/", "archivos", "a.csv")
	ls := lineas(buf)
	if len(ls) > 13 {
		t.Fatalf("%d líneas:\n%s", len(ls), buf.String())
	}
	if strings.Contains(buf.String(), "archivado") || strings.Contains(buf.String(), "evaluado") {
		t.Errorf("los registros por ticker no van a la consola:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "[Asia] [sector]") || !strings.Contains(buf.String(), "20/20") {
		t.Errorf("falta la barra de sectores:\n%s", buf.String())
	}
	if !strings.Contains(ls[len(ls)-1], "instalados en current/") {
		t.Errorf("los hitos sí se muestran: %q", ls[len(ls)-1])
	}
}

func TestConsolaDosRecorridosSeparadosReiniciaLaBarra(t *testing.T) {
	log, buf := logConsola(slog.LevelInfo)
	for round := 0; round < 2; round++ {
		for i := 1; i <= 30; i++ {
			log.Info("indicadores", "ticker", "T", "n", fmt.Sprintf("%d/30", i))
		}
	}
	if got := strings.Count(buf.String(), "1/30"); got < 2 { // "1/30" aparece en ambos recorridos (y en 11/30…, de ahí ≥)
		t.Errorf("el segundo recorrido debe mostrarse desde el inicio:\n%s", buf.String())
	}
	if got := strings.Count(buf.String(), " 30/30"); got != 2 {
		t.Errorf("cada recorrido termina en 30/30: %d\n%s", got, buf.String())
	}
}

func TestConsolaDebugMuestraTodo(t *testing.T) {
	log, buf := logConsola(slog.LevelDebug)
	for i := 1; i <= 50; i++ {
		log.Info("ETF", "n", fmt.Sprintf("%d/50", i), "ticker", "X")
	}
	log.Info("archivado: x.csv")
	if got := len(lineas(buf)); got != 51 {
		t.Errorf("con LOG_LEVEL=DEBUG se muestra todo: %d líneas, quería 51", got)
	}
}

func TestConsolaCortaValoresLargos(t *testing.T) {
	log, buf := logConsola(slog.LevelInfo)
	log.Info("instalados", "archivos", strings.Repeat("archivo.csv, ", 60))
	l := lineas(buf)[0]
	if !strings.Contains(l, "…") || len([]rune(l)) > 220 {
		t.Errorf("el valor largo debe cortarse con «…» (%d caracteres): %q", len([]rune(l)), l)
	}
}

func TestConsolaRespetaElNivel(t *testing.T) {
	log, buf := logConsola(slog.LevelWarn)
	log.Info("no se ve")
	log.Warn("se ve")
	if got := lineas(buf); len(got) != 1 || !strings.Contains(got[0], "se ve") {
		t.Errorf("%v", got)
	}
}

func TestConsolaEncabezadoDeModuloYSinRuido(t *testing.T) {
	log, buf := logConsola(slog.LevelInfo)
	log.Info("inicio", "modulo", "cl_stocks", "intento", "1/1", "opciones", "map[]")
	log.Info("OK", "modulo", "cl_stocks", "intento", 1, "duracion_s", 19.7, "mensaje", "30/30 tickers · 93 requests a Yahoo")
	log.Info("resultado", "modulo", "cl_stocks", "estado", "success")
	ls := lineas(buf)
	if len(ls) != 3 || ls[0] != "" { // línea en blanco + encabezado + OK
		t.Fatalf("%d líneas: %q", len(ls), buf.String())
	}
	if !strings.Contains(ls[1], ">>> cl_stocks  intento=1/1") || strings.Contains(ls[1], "opciones") {
		t.Errorf("encabezado del módulo: %q", ls[1])
	}
	if !strings.Contains(ls[2], "OK  modulo=cl_stocks") || strings.Contains(ls[2], "mensaje") || strings.Contains(buf.String(), "resultado") {
		t.Errorf("el OK no repite el mensaje ni sale «resultado»: %q", ls[2])
	}
}

func TestConsolaBarraNoRetrocedeConHilosDesordenados(t *testing.T) {
	log, buf := logConsola(slog.LevelInfo)
	for _, n := range []int{3, 1, 4, 2, 6, 5, 9, 12, 10, 30, 29} { // llegan fuera de orden y los últimos son atrasados
		log.Info("indicadores", "ticker", "T", "n", fmt.Sprintf("%d/30", n))
	}
	var vistos []string
	for _, l := range lineas(buf) {
		vistos = append(vistos, strings.Fields(l[strings.LastIndex(l, "]")+1:])[1])
	}
	if got := strings.Join(vistos, " "); got != "3/30 6/30 9/30 12/30 30/30" {
		t.Errorf("la barra solo avanza: %q en %q", got, buf.String())
	}
}

func TestConsolaPartePorLineaLosAvisosLargos(t *testing.T) {
	var buf bytes.Buffer
	h := newConsoleHandler(&buf, slog.LevelInfo)
	lista := strings.Repeat("TSM (Taiwan), ", 20) + "BHP (Australia)"
	msg := "[EEUU] excluidas 21 empresas con sede fuera de EEUU (ADR/instrumento en USD de una empresa extranjera): " + lista
	_ = h.Handle(context.Background(), slog.NewRecord(time.Date(2026, 10, 21, 18, 33, 3, 0, time.UTC), slog.LevelWarn, msg, 0))
	ls := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(ls) < 3 || !strings.HasPrefix(ls[0], "18:33:03  AVISO  [EEUU] excluidas 21 empresas") || !strings.HasSuffix(ls[0], "extranjera):") {
		t.Fatalf("el encabezado va solo en la primera línea:\n%s", buf.String())
	}
	for _, l := range ls[1:] {
		if !strings.HasPrefix(l, strings.Repeat(" ", 19)) || len([]rune(l)) > 120 {
			t.Errorf("las continuaciones van sangradas y acotadas: %q", l)
		}
	}
	if !strings.Contains(strings.Join(ls, " "), "BHP (Australia)") {
		t.Error("no debe perderse texto")
	}
}
