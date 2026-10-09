package yahoo

import (
	"context"

	"github.com/loc1263/gohoo"
)

// InfoModules son los módulos de quoteSummary que alimentan Info.
var InfoModules = []string{"price", "summaryDetail", "defaultKeyStatistics", "financialData", "assetProfile", "quoteType"}

// infoPriority: orden en que se busca cada clave (el primer módulo que la trae gana). Se fijó
// comparando con la info del ticker de Yahoo: p. ej. forwardPE sale de summaryDetail
// (el de defaultKeyStatistics viene corrupto en algunas acciones chilenas: 11454 en vez de 10.8).
var infoPriority = []string{"summaryDetail", "price", "defaultKeyStatistics", "financialData", "assetProfile", "quoteType"}

// Info aplana quoteSummary en un mapa clave→valor como la info del ticker de Yahoo (números ya
// sin el envoltorio {raw, fmt}). Ver FlattenInfo para las reglas de mapeo.
func Info(ctx context.Context, src Source, ticker string) (map[string]any, error) {
	qs, err := src.QuoteSummary(ctx, ticker, InfoModules)
	if err != nil {
		return nil, err
	}
	return FlattenInfo(qs), nil
}

// FlattenInfo implementa el mapeo info ← quoteSummary.
func FlattenInfo(qs map[string]any) map[string]any {
	out := map[string]any{}
	for _, mod := range infoPriority {
		m, _ := qs[mod].(map[string]any)
		for k, v := range m {
			if _, taken := out[k]; taken {
				continue
			}
			if rv := gohoo.RawValue(v); rv != nil {
				out[k] = rv
			}
		}
	}
	// El dividendYield se entrega en porcentaje (0.32) y Yahoo en fracción (0.0032). En ETFs
	// summaryDetail no trae dividendYield, pero sí «yield».
	if f, ok := gohoo.AsFloat(out["dividendYield"]); ok {
		out["dividendYield"] = f * 100
	} else if f, ok := gohoo.AsFloat(out["yield"]); ok {
		out["dividendYield"] = f * 100
	}
	return out
}
