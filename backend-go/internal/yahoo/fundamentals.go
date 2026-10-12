package yahoo

import (
	"context"

	"github.com/loc1263/gohoo"
)

// Point es una observación de fundamentals-timeseries.
type Point = gohoo.Point

// TimeseriesPoints extrae las observaciones de un tipo (p. ej. «quarterlyTotalRevenue»), la más
// reciente primero y sin faltantes (como .dropna() sobre quarterly_income_stmt).
func TimeseriesPoints(data map[string]any, typ string) []Point {
	return gohoo.TimeseriesPoints(data, typ)
}

// TrailingPEG es el último trailingPegRatio de fundamentals-timeseries (quoteSummary no lo trae).
// Va por la Source para poder probar los módulos con falsos. ok=false si Yahoo no tiene el dato.
func TrailingPEG(ctx context.Context, src Source, ticker string) (v float64, ok bool, err error) {
	data, err := src.Fundamentals(ctx, ticker, []string{"trailingPegRatio"})
	if err != nil {
		return 0, false, err
	}
	pts := gohoo.TimeseriesPoints(data, "trailingPegRatio")
	if len(pts) == 0 {
		return 0, false, nil
	}
	return pts[0].Value, true, nil // más reciente primero
}
