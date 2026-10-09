package yahoo

import (
	"context"
	"sort"

	"github.com/loc1263/gohoo"
)

// Point es una observación de fundamentals-timeseries.
type Point struct {
	Date  string // asOfDate «YYYY-MM-DD»
	Value float64
}

// TimeseriesPoints extrae de una respuesta de FundamentalsTimeseries las observaciones del tipo dado
// (p. ej. «quarterlyTotalRevenue»), la más reciente primero y sin valores faltantes (como .dropna()
// sobre las columnas de quarterly_income_stmt, que también vienen de más nueva a más vieja).
func TimeseriesPoints(data map[string]any, typ string) []Point {
	var out []Point
	res, _ := gohoo.GetPath(data, "timeseries", "result").([]any)
	for _, it := range res {
		m, _ := it.(map[string]any)
		arr, _ := m[typ].([]any)
		for _, e := range arr {
			em, _ := e.(map[string]any)
			if em == nil {
				continue
			}
			date, _ := em["asOfDate"].(string)
			v, ok := gohoo.AsFloat(gohoo.GetPath(em, "reportedValue", "raw"))
			if date == "" || !ok {
				continue
			}
			out = append(out, Point{date, v})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out
}

// TrailingPEG es el último trailingPegRatio de fundamentals-timeseries (se agrega a la info del ticker
// desde ese endpoint; quoteSummary no lo trae). ok=false si Yahoo no tiene el dato.
func TrailingPEG(ctx context.Context, src Source, ticker string) (v float64, ok bool, err error) {
	data, err := src.Fundamentals(ctx, ticker, []string{"trailingPegRatio"})
	if err != nil {
		return 0, false, err
	}
	pts := TimeseriesPoints(data, "trailingPegRatio")
	if len(pts) == 0 {
		return 0, false, nil
	}
	return pts[0].Value, true, nil // más reciente primero
}
