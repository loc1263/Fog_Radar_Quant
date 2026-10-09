package us

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/loc1263/gohoo"

	"fogradar/internal/core"
	"fogradar/internal/yahoo"
)

// TickerNotFoundError: el ticker no existe o Yahoo Finance no devolvió datos utilizables.
type TickerNotFoundError struct{ Msg string }

func (e *TickerNotFoundError) Error() string { return e.Msg }

// EvalTTL y EvalMax: caché de la evaluación en vivo (mismo ticker en < 5 min no vuelve a Yahoo).
const (
	EvalTTL = 5 * time.Minute
	EvalMax = 256
)

func strOr(info map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := info[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// EvaluateTicker consulta Yahoo en vivo para UN ticker (única llamada a Yahoo dentro de un request:
// el contexto debe venir marcado con yahoo.Interactive para no dormir en un backoff largo).
func EvaluateTicker(ctx context.Context, src yahoo.Source, ticker string) (*TickerEvaluation, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if ticker == "" {
		return nil, &TickerNotFoundError{"Ticker vacío"}
	}
	info, err := yahoo.Info(ctx, src, ticker)
	if err != nil {
		if errors.Is(err, gohoo.ErrRateLimit) || ctx.Err() != nil {
			return nil, err
		}
		return nil, &TickerNotFoundError{fmt.Sprintf("No se pudo obtener información de %s: %v", ticker, err)}
	}
	if v, ok, perr := yahoo.TrailingPEG(ctx, src, ticker); perr == nil && ok {
		info["trailingPegRatio"] = v
	}
	h, err := src.Chart(ctx, ticker, gohoo.ChartParams{Range: "1y", Interval: "1d"})
	if err != nil && (errors.Is(err, gohoo.ErrRateLimit) || ctx.Err() != nil) {
		return nil, err
	}
	if err != nil || len(h.Bars) == 0 {
		return nil, &TickerNotFoundError{fmt.Sprintf("'%s' no parece ser un ticker válido o no tiene histórico disponible", ticker)}
	}
	bars := yahoo.AutoAdjust(h.Bars)
	var closes, vols []float64
	for _, b := range bars {
		if !math.IsNaN(b.Close) {
			closes = append(closes, b.Close)
		}
		vols = append(vols, b.Volume)
	}

	price := num(info["currentPrice"])
	if !truthy(price) {
		price = num(info["regularMarketPrice"])
	}
	if !truthy(price) {
		price = nil
	}
	if price == nil && len(closes) > 0 {
		p := closes[len(closes)-1]
		price = &p
	}
	mc := num(info["marketCap"])

	ev := &TickerEvaluation{
		Ticker: ticker, Name: strOr(info, "longName", "shortName"), Sector: strOr(info, "sector"),
		Industry: strOr(info, "industry"), Currency: strOr(info, "currency"), Price: price, MarketCap: mc,
		FetchedAt: core.Fecha{Time: time.Now().UTC()},
		Valuation: BuildValuation(info), Profitability: BuildProfitability(info),
		FinancialHealth: BuildFinancialHealth(info), ShareholderReturn: BuildShareholderReturn(info),
		Technical: BuildTechnical(closes, vols),
	}
	if ev.Name == "" {
		ev.Name = ticker
	}
	ev.Score = EvaluateScore(Evaluation{Price: ev.Price, MarketCap: ev.MarketCap, Valuation: ev.Valuation,
		Profitability: ev.Profitability, FinancialHealth: ev.FinancialHealth, ShareholderReturn: ev.ShareholderReturn,
		Technical: ev.Technical})
	return ev, nil
}

// EvalCache es la caché TTL de evaluaciones.
type EvalCache struct {
	mu   sync.Mutex
	data map[string]evalEntry
	now  func() time.Time
}

type evalEntry struct {
	at time.Time
	v  TickerEvaluation
}

// NewEvalCache crea la caché.
func NewEvalCache() *EvalCache { return &EvalCache{data: map[string]evalEntry{}, now: time.Now} }

// Get devuelve la evaluación si está vigente.
func (c *EvalCache) Get(k string) (TickerEvaluation, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data[k]
	if !ok || c.now().Sub(e.at) >= EvalTTL {
		delete(c.data, k)
		return TickerEvaluation{}, false
	}
	return e.v, true
}

// Put guarda una evaluación (si se llena, descarta las vencidas y luego la más vieja).
func (c *EvalCache) Put(k string, v TickerEvaluation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.data) >= EvalMax {
		oldest, oldestAt, have := "", time.Time{}, false
		for key, e := range c.data {
			if c.now().Sub(e.at) >= EvalTTL {
				delete(c.data, key)
			} else if !have || e.at.Before(oldestAt) {
				oldest, oldestAt, have = key, e.at, true
			}
		}
		if len(c.data) >= EvalMax && have {
			delete(c.data, oldest)
		}
	}
	c.data[k] = evalEntry{c.now(), v}
}

// Clear vacía la caché (tests).
func (c *EvalCache) Clear() {
	c.mu.Lock()
	c.data = map[string]evalEntry{}
	c.mu.Unlock()
}
