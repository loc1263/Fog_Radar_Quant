//go:build live

package etf

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"fogradar/internal/core"
	"fogradar/internal/yahoo"
)

// go test -tags live -run TestLiveCache -v ./internal/modules/etf — 16 ETFs reales, dos veces (la 2ª debe usar la caché).
func TestLiveCache(t *testing.T) {
	st, _ := core.LoadSettings()
	tr := yahoo.NewTraffic(yahoo.PolicyFromSettings(st))
	c := yahoo.NewClient(tr)
	lista := []byte("Ticker;Name;Company;Category\n")
	for _, tk := range []string{"SPY", "QQQ", "VTI", "XYLD", "CRAK", "PBP", "SGVT", "BND", "GLD", "ARKK", "TLT", "SCHD", "IWM", "EEM", "VBIL", "GTOS"} {
		lista = append(lista, []byte(tk+";;;\n")...)
	}
	dir := t.TempDir()
	o := ScreenerOpts{OutputDir: dir, ListaCSV: lista, Concurrencia: 4, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for i := 1; i <= 2; i++ {
		tr.ResetStats()
		res, err := ProcesarETFs(context.Background(), c, o)
		if err != nil || res == nil {
			t.Fatal(err)
		}
		t.Logf("corrida %d: %s · %s", i, res.CSV, tr.Stats().Summary())
	}
}
