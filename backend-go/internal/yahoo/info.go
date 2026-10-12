package yahoo

import (
	"context"

	"github.com/loc1263/gohoo"
)

// Info aplana quoteSummary en un mapa clave→valor como la info del ticker de Yahoo (ver FlattenInfo).
// Va por la Source (y no por gohoo.Client.Info) para que los módulos se puedan probar con falsos.
func Info(ctx context.Context, src Source, ticker string) (map[string]any, error) {
	qs, err := src.QuoteSummary(ctx, ticker, gohoo.InfoModules())
	if err != nil {
		return nil, err
	}
	return FlattenInfo(qs), nil
}

// FlattenInfo es gohoo.FlattenInfo con dividendYield en porcentaje (como lo entrega la info del
// ticker; en ETFs, desde «yield»). La prioridad de módulos (summaryDetail primero, por el forwardPE
// corrupto de defaultKeyStatistics en acciones chilenas) la fija gohoo.InfoModules.
func FlattenInfo(qs map[string]any) map[string]any {
	return gohoo.FlattenInfo(qs, gohoo.InfoPercentYield())
}
