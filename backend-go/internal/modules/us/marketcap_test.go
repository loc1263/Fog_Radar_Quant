package us

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/loc1263/gohoo"
)

// srcSinLote es una fuente sin cotizaciones en lote.
type srcSinLote struct{}

func (srcSinLote) Chart(context.Context, string, gohoo.ChartParams) (*gohoo.ChartResult, error) {
	return nil, nil
}
func (srcSinLote) QuoteSummary(context.Context, string, []string) (map[string]any, error) {
	return nil, nil
}
func (srcSinLote) Fundamentals(context.Context, string, []string) (map[string]any, error) {
	return nil, nil
}

// srcConLote agrega Quotes: AAPL y MSFT con market cap, ^IDX sin él; DESCONOCIDO no vuelve.
type srcConLote struct {
	srcSinLote
	calls int
	err   error
}

func (s *srcConLote) Quotes(_ context.Context, symbols []string) ([]gohoo.Quote, error) {
	s.calls++
	return []gohoo.Quote{{Symbol: "MSFT", MarketCap: 4e12}, {Symbol: "AAPL", MarketCap: 5e12}, {Symbol: "^IDX"}}, s.err
}

func silentLog() *runLog {
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &runLog{console: l, file: l}
}

func TestBatchMarketCapsEnLoteYPendientes(t *testing.T) {
	src := &srcConLote{}
	caps, pending := batchMarketCaps(context.Background(), src, []string{"AAPL", "DESCONOCIDO", "MSFT", "^IDX"}, silentLog())
	if src.calls != 1 {
		t.Fatalf("debe pedir todo en una llamada: %d", src.calls)
	}
	if caps[0] == nil || *caps[0] != 5e12 || caps[2] == nil || *caps[2] != 4e12 || caps[1] != nil || caps[3] != nil {
		t.Fatalf("caps = %v", caps)
	}
	// los que el lote no trae (o trae sin market cap) quedan para quoteSummary
	if len(pending) != 2 || pending[0] != 1 || pending[1] != 3 {
		t.Fatalf("pendientes = %v", pending)
	}
}

func TestBatchMarketCapsSinLoteOConError(t *testing.T) {
	univ := []string{"AAPL", "MSFT"}
	if _, pending := batchMarketCaps(context.Background(), srcSinLote{}, univ, silentLog()); len(pending) != 2 {
		t.Fatalf("sin lote todo queda pendiente: %v", pending)
	}
	// Con error se usan los resultados parciales del último intento y el resto queda pendiente.
	src := &srcConLote{err: errors.New("lote 2 falló")}
	caps, pending := batchMarketCaps(context.Background(), src, []string{"AAPL", "TSLA"}, silentLog())
	if caps[0] == nil || len(pending) != 1 || pending[0] != 1 || src.calls != MaxRetries+1 {
		t.Fatalf("caps=%v pendientes=%v llamadas=%d", caps, pending, src.calls)
	}
}
