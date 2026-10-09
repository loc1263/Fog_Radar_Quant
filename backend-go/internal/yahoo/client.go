package yahoo

import (
	"context"
	"sync"
	"time"

	"github.com/loc1263/gohoo"
)

// Source es lo que usan los módulos para hablar con Yahoo. Los tests la reemplazan por un falso.
type Source interface {
	// Chart descarga barras (rango relativo o fechas).
	Chart(ctx context.Context, ticker string, p gohoo.ChartParams) (*gohoo.ChartResult, error)
	// QuoteSummary pide módulos de quoteSummary y devuelve el árbol crudo.
	QuoteSummary(ctx context.Context, ticker string, modules []string) (map[string]any, error)
	// Fundamentals pide tipos de fundamentals-timeseries (p. ej. quarterlyTotalRevenue).
	Fundamentals(ctx context.Context, ticker string, types []string) (map[string]any, error)
}

// backend es la parte de *gohoo.Client que usamos (permite inyectar un falso en los tests).
type backend interface {
	ChartRange(ctx context.Context, ticker string, p gohoo.ChartParams) (*gohoo.ChartResult, error)
	QuoteSummaryContext(ctx context.Context, ticker string, modules []string) (map[string]any, error)
	FundamentalsTimeseriesContext(ctx context.Context, ticker string, types []string) (map[string]any, error)
}

// Client implementa Source sobre gohoo con el control de tráfico común.
type Client struct {
	T *Traffic

	mu   sync.Mutex
	raw  backend
	make func() (backend, error)
}

// NewClient crea un cliente real. La sesión (cookie + crumb) se negocia en el primer request, y si
// falla se reintenta en el siguiente (no se guarda el error).
func NewClient(t *Traffic) *Client {
	return &Client{T: t, make: func() (backend, error) {
		return gohoo.NewClientWithOptions(gohoo.WithRetryPolicy(gohoo.RetryPolicy{
			MaxAttempts:     t.P.NetRetries + 1,
			BaseDelay:       200 * time.Millisecond,
			MaxDelay:        5 * time.Second,
			Jitter:          0.3,
			RetryRateLimit:  false, // los 429 los maneja Traffic con su backoff largo
			HonorRetryAfter: true,
		}))
	}}
}

// NewClientWith crea un cliente sobre un backend dado (tests).
func newClientWith(t *Traffic, b backend) *Client { return &Client{T: t, raw: b} }

func (c *Client) backend() (backend, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.raw != nil {
		return c.raw, nil
	}
	b, err := c.make()
	if err != nil {
		return nil, err
	}
	c.raw = b
	return b, nil
}

// Chart implementa Source.
func (c *Client) Chart(ctx context.Context, ticker string, p gohoo.ChartParams) (*gohoo.ChartResult, error) {
	var out *gohoo.ChartResult
	err := c.T.Do(ctx, "chart/"+ticker, func() error {
		b, err := c.backend()
		if err != nil {
			return err
		}
		out, err = b.ChartRange(ctx, ticker, p)
		return err
	})
	return out, err
}

// QuoteSummary implementa Source.
func (c *Client) QuoteSummary(ctx context.Context, ticker string, modules []string) (map[string]any, error) {
	var out map[string]any
	err := c.T.Do(ctx, "quoteSummary/"+ticker, func() error {
		b, err := c.backend()
		if err != nil {
			return err
		}
		out, err = b.QuoteSummaryContext(ctx, ticker, modules)
		return err
	})
	return out, err
}

// Fundamentals implementa Source.
func (c *Client) Fundamentals(ctx context.Context, ticker string, types []string) (map[string]any, error) {
	var out map[string]any
	err := c.T.Do(ctx, "fundamentals/"+ticker, func() error {
		b, err := c.backend()
		if err != nil {
			return err
		}
		out, err = b.FundamentalsTimeseriesContext(ctx, ticker, types)
		return err
	})
	return out, err
}
