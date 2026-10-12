package yahoo

import (
	"context"
	"fmt"

	"github.com/loc1263/gohoo"
)

// Screener de Yahoo sobre gohoo.Screen (misma sesión, espaciado y reintentos que el resto de las llamadas).

// ScreenParams es una consulta al screener (la query se arma con gohoo.Eq, gohoo.And, ...).
type ScreenParams = gohoo.ScreenParams

// ErrEmptyScreen: Yahoo respondió sin quotes (cambio de API o región vacía).
var ErrEmptyScreen = gohoo.ErrEmptyScreen

// Quotes son los resultados crudos de Yahoo (números ya sin el envoltorio {raw, fmt}).
type Quotes []map[string]any

// Screen consulta el screener de Yahoo y devuelve los quotes crudos. Una página sin quotes
// devuelve ErrEmptyScreen (para el descubrimiento de universo, una región vacía es una falla).
func (c *Client) Screen(ctx context.Context, p ScreenParams) (Quotes, error) {
	b, ctx, err := c.call(ctx)
	if err != nil {
		return nil, err
	}
	res, err := b.Screen(ctx, p)
	if err != nil {
		return nil, err
	}
	if len(res.Quotes) == 0 {
		return nil, fmt.Errorf("screener sin quotes (posible cambio de API o región vacía): %w", ErrEmptyScreen)
	}
	out := make(Quotes, len(res.Quotes))
	for i, q := range res.Quotes {
		out[i] = q.Raw
	}
	return out, nil
}
