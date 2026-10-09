package yahoo

import "testing"

func TestFlattenInfoPrioridadYDividendYield(t *testing.T) {
	qs := map[string]any{
		"summaryDetail":        map[string]any{"forwardPE": map[string]any{"raw": 10.8, "fmt": "10.8"}, "dividendYield": map[string]any{"raw": 0.0279}},
		"defaultKeyStatistics": map[string]any{"forwardPE": map[string]any{"raw": 11454.7}, "beta": 1.2},
		"price":                map[string]any{"shortName": "COPEC", "empty": nil},
	}
	info := FlattenInfo(qs)
	if info["forwardPE"] != 10.8 {
		t.Fatalf("summaryDetail gana: %v", info["forwardPE"])
	}
	if y := info["dividendYield"].(float64); y < 2.78 || y > 2.80 {
		t.Fatalf("dividendYield en porcentaje: %v", y)
	}
	if info["shortName"] != "COPEC" || info["beta"] != 1.2 {
		t.Fatalf("%v", info)
	}
	if _, ok := info["empty"]; ok {
		t.Fatal("los null no se incluyen")
	}
	// ETF: sin dividendYield, se usa «yield»
	etf := FlattenInfo(map[string]any{"summaryDetail": map[string]any{"yield": map[string]any{"raw": 0.0099}}})
	if y := etf["dividendYield"].(float64); y < 0.98 || y > 1.0 {
		t.Fatalf("ETF: %v", y)
	}
	if _, ok := FlattenInfo(map[string]any{})["dividendYield"]; ok {
		t.Fatal("sin datos no hay dividendYield")
	}
}
