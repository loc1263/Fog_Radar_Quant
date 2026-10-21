package yahoo

import (
	"context"
	"sync"

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

// Quoter es una Source que además da cotizaciones en lote (lo cumple *Client). Los módulos lo
// detectan con una aserción de tipo, así los falsos de los tests no están obligados a implementarlo.
type Quoter interface {
	Quotes(ctx context.Context, symbols []string) ([]gohoo.Quote, error)
}

// backend es la parte de *gohoo.Client que usamos (permite inyectar un falso en los tests).
type backend interface {
	ChartRange(ctx context.Context, ticker string, p gohoo.ChartParams) (*gohoo.ChartResult, error)
	QuoteSummaryContext(ctx context.Context, ticker string, modules []string) (map[string]any, error)
	FundamentalsTimeseriesContext(ctx context.Context, ticker string, types []string) (map[string]any, error)
	Screen(ctx context.Context, p gohoo.ScreenParams) (*gohoo.ScreenResult, error)
	Quotes(ctx context.Context, symbols []string) ([]gohoo.Quote, error)
}

// Client implementa Source sobre gohoo con el control de tráfico común (instalado en gohoo).
type Client struct {
	T *Traffic

	mu   sync.Mutex
	raw  backend
	make func() (backend, error)
}

// NewClient crea un cliente real. La sesión (cookie + crumb) se negocia en el primer request que
// la necesita, y si falla se reintenta en el siguiente (no se guarda el error).
func NewClient(t *Traffic) *Client {
	return &Client{T: t, make: func() (backend, error) { return gohoo.NewClientWithOptions(t.options()...) }}
}

// options instala las reglas de tráfico en un cliente de gohoo.
func (t *Traffic) options() []gohoo.Option {
	return []gohoo.Option{
		gohoo.WithLazyBootstrap(),
		gohoo.WithRetryPolicy(t.retryPolicy()),
		gohoo.WithLimiter(t.limiter()),
		gohoo.WithRequestHook(t.onRequest),
	}
}

// newClientWith crea un cliente sobre un backend dado (tests).
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

// call prepara el backend y el contexto.
func (c *Client) call(ctx context.Context) (backend, context.Context, error) {
	b, err := c.backend()
	if err != nil {
		return nil, ctx, err
	}
	return b, ctx, nil
}

// Chart implementa Source.
func (c *Client) Chart(ctx context.Context, ticker string, p gohoo.ChartParams) (*gohoo.ChartResult, error) {
	b, ctx, err := c.call(ctx)
	if err != nil {
		return nil, err
	}
	return b.ChartRange(ctx, ticker, p)
}

// QuoteSummary implementa Source.
func (c *Client) QuoteSummary(ctx context.Context, ticker string, modules []string) (map[string]any, error) {
	b, ctx, err := c.call(ctx)
	if err != nil {
		return nil, err
	}
	return b.QuoteSummaryContext(ctx, ticker, modules)
}

// Fundamentals implementa Source.
func (c *Client) Fundamentals(ctx context.Context, ticker string, types []string) (map[string]any, error) {
	b, ctx, err := c.call(ctx)
	if err != nil {
		return nil, err
	}
	return b.FundamentalsTimeseriesContext(ctx, ticker, types)
}

// Quotes implementa Quoter: cotizaciones en lotes de gohoo.QuoteBatchSize (los símbolos que Yahoo
// no conoce no aparecen). Si un lote falla, devuelve los anteriores junto con el error.
func (c *Client) Quotes(ctx context.Context, symbols []string) ([]gohoo.Quote, error) {
	b, ctx, err := c.call(ctx)
	if err != nil {
		return nil, err
	}
	return b.Quotes(ctx, symbols)
}
