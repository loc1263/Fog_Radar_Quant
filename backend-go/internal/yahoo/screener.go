package yahoo

import (
	"context"
	"fmt"

	"github.com/loc1263/gohoo"
)

// Screener de Yahoo sobre gohoo.Screen (misma sesión cookie + crumb que el resto de las llamadas),
// con el espaciado y el backoff ante 429 de Traffic.

// ScreenParams es una consulta al screener (la query se arma con gohoo.Eq, gohoo.And, ...).
type ScreenParams = gohoo.ScreenParams

// ErrEmptyScreen: Yahoo respondió sin quotes (cambio de API o región vacía).
var ErrEmptyScreen = gohoo.ErrEmptyScreen

// Quotes son los resultados crudos de Yahoo (números ya sin el envoltorio {raw, fmt}).
type Quotes []map[string]any

// Screen consulta el screener de Yahoo y devuelve los quotes crudos. Una página sin quotes
// devuelve ErrEmptyScreen (para el descubrimiento de universo, una región vacía es una falla).
func (c *Client) Screen(ctx context.Context, p ScreenParams) (Quotes, error) {
	var res *gohoo.ScreenResult
	err := c.T.Do(ctx, "screener", func() error {
		b, err := c.backend()
		if err != nil {
			return err
		}
		res, err = b.Screen(ctx, p)
		return err
	})
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
