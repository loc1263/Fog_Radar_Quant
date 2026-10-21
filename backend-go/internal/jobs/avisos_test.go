package jobs

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestHMS(t *testing.T) {
	for d, want := range map[time.Duration]string{0: "00:00:00", 43 * time.Second: "00:00:43", 1164 * time.Second: "00:19:24", 25*time.Hour + 61*time.Second: "25:01:01", 1500 * time.Millisecond: "00:00:02"} {
		if got := HMS(d); got != want {
			t.Errorf("HMS(%v) = %s, quería %s", d, got, want)
		}
	}
}

func TestConAvisosYResumen(t *testing.T) {
	base := slog.New(slog.NewTextHandler(io.Discard, nil))
	log, tomar := conAvisos(base)
	log.Info("no cuenta")
	log.Warn("sin histórico", "ticker", "ABC", "error", "HTTP 404")
	log.Warn("sin histórico", "ticker", "ABC", "error", "HTTP 404") // repetido exacto: una sola vez
	log.Error("algo general")
	av := tomar()
	if len(av) != 2 || av[0].Ticker != "ABC" || !strings.Contains(av[0].Mensaje, "HTTP 404") {
		t.Fatalf("%+v", av)
	}
	if len(tomar()) != 0 {
		t.Error("tomar debía vaciar los avisos")
	}

	t0 := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	out := Resumen([]Resultado{
		{Modulo: "cl_stocks", Estado: EstadoOK, Duracion: 43 * time.Second, Mensaje: "ok", Inicio: t0, Fin: t0.Add(43 * time.Second), Avisos: av},
		{Modulo: "etf", Estado: EstadoOK, Duracion: 19 * time.Minute, Inicio: t0.Add(time.Minute), Fin: t0.Add(20 * time.Minute)},
		{Modulo: "us_stocks", Estado: EstadoOmitido},
	})
	for _, want := range []string{"00:00:43", "00:19:00", "Tiempo total: 00:20:00", "cl_stocks (2)", "ABC", "· algo general"} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q en:\n%s", want, out)
		}
	}
	if !strings.Contains(Resumen([]Resultado{{Modulo: "x", Estado: EstadoOK}}), "Sin avisos.") {
		t.Error("sin avisos debía decirlo")
	}
}
