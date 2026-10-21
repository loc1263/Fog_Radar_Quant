package tend

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"fogradar/internal/quant"
)

// Estimación de tendencia a 6 meses (26 semanas) para las acciones seleccionadas:
//  1. Señal técnica: cruce de medias móviles semanales (10 vs 40; «Señal 3m» 4 vs 13). Solo informativa.
//  2. Proyección tipo GBM con el CAGR histórico (mu) y la volatilidad anual (sigma): mediana = P·(1+mu)^T, banda 90 % =
//     mediana·exp(±1.645·sigma·√T).
//  3. Notas de régimen de mercado (VIX, curva 10Y-2Y y spread de crédito High Yield) si hay datos.

// Z90 es el z-score de la banda de confianza ~90 %.
const Z90 = 1.645

// TrendSignal: cruce de medias móviles semanales. Devuelve la señal y las dos medias (NaN si no hay datos suficientes).
func TrendSignal(s []float64, shortW, longW int) (string, float64, float64) {
	ma := func(w int) float64 {
		if len(s) < w {
			return quant.NaN
		}
		sum := 0.0
		for _, v := range s[len(s)-w:] {
			sum += v
		}
		return sum / float64(w)
	}
	sm, lm := ma(shortW), ma(longW)
	last := s[len(s)-1]
	switch {
	case math.IsNaN(sm) || math.IsNaN(lm):
		return "Datos insuficientes", sm, lm
	case sm > lm && last >= sm:
		return "Alcista", sm, lm
	case sm < lm && last <= sm:
		return "Bajista", sm, lm
	}
	return "Lateral / mixta", sm, lm
}

// ProjectPrice: proyección GBM. mu es el CAGR; devuelve (mediana, percentil 5, percentil 95) a horizonYears.
func ProjectPrice(last, mu, sigma, horizonYears, z float64) (central, low, high float64) {
	drift := math.Log1p(mu) * horizonYears
	sh := sigma * math.Sqrt(horizonYears)
	return last * math.Exp(drift), last * math.Exp(drift-z*sh), last * math.Exp(drift+z*sh)
}

func dropNaN(s []float64) []float64 { return quant.DropNaN(s) }

// ForecastTable calcula la tabla de proyección para los tickers (ordenada por retorno esperado a 6 meses, descendente).
func ForecastTable(prices *Prices, top []string, u *Universe, p Params) *quant.Frame {
	pt := prices.Subset(top)
	rets := quant.ReturnsFromPrices(pt.Cols)
	mu := quant.MeanHistoricalReturn(rets, float64(p.WeeksPerYear))
	S := quant.SampleCov(rets, float64(p.WeeksPerYear))
	horizon := float64(p.HorizonWeeks) / float64(p.WeeksPerYear)

	type row struct {
		cells []any
		ret6m float64
	}
	var rows []row
	for j, t := range top {
		s := dropNaN(pt.Cols[j])
		last := s[len(s)-1]
		sig, sm, lm := TrendSignal(s, p.ShortMA, p.LongMA)
		sig3, sm3, lm3 := TrendSignal(s, p.ShortMA3m, p.LongMA3m)
		mom := quant.NaN
		if len(s) > p.MomentumLookbackWeeks {
			mom = s[len(s)-1]/s[len(s)-p.MomentumLookbackWeeks-1] - 1
		}
		mui, sigi := mu[j], math.Sqrt(S.At(j, j))
		c, lo, hi := ProjectPrice(last, mui, sigi, horizon, Z90)
		r2 := func(x float64) float64 { return quant.Round(x, 2) }
		nr := func(x float64) any {
			if math.IsNaN(x) {
				return nil
			}
			return r2(x)
		}
		ret6 := r2((c/last - 1) * 100)
		rows = append(rows, row{ret6m: ret6, cells: []any{
			t, u.Info[t].Name, u.Info[t].Country, u.SectorOf(t), r2(last), sig, nr(sm), nr(lm), sig3, nr(sm3), nr(lm3),
			nr(mom * 100), r2(mui * 100), r2(sigi * 100), r2(c), r2(lo), r2(hi), ret6}})
	}
	vals := make([]float64, len(rows))
	for i, r := range rows {
		vals[i] = r.ret6m
	}
	ord := quant.OrderDesc(vals)
	names := []string{"ticker", "nombre", "pais", "sector", "precio_actual_usd", "senal_tecnica", "ma_10s", "ma_40s",
		"senal_tecnica_3m", "ma_4s", "ma_13s", "momentum_26s_pct", "retorno_esperado_anual_pct", "volatilidad_anual_pct",
		"precio_proyectado_6m_usd", "precio_min_90pct_6m_usd", "precio_max_90pct_6m_usd", "retorno_esperado_6m_pct"}
	f := quant.NewFrame(len(rows))
	for k, n := range names {
		col := make([]any, len(rows))
		for i, o := range ord {
			col[i] = rows[o].cells[k]
		}
		f.Set(quant.Cells(n, col))
	}
	return f
}

// RegimeNotes arma las notas de régimen de mercado (VIX del caché de precios y macro_data_*.csv).
func RegimeNotes(cacheDir string, warn func(string, ...any)) []string {
	var notes []string
	if bars, err := readCloseCSV(filepath.Join(cacheDir, "prices", "_VIX.csv")); err == nil && len(bars) > 0 {
		vix := bars[len(bars)-1]
		switch {
		case vix >= 25:
			notes = append(notes, fmt.Sprintf("VIX en %.1f: volatilidad elevada, entorno risk-off.", vix))
		case vix <= 15:
			notes = append(notes, fmt.Sprintf("VIX en %.1f: volatilidad baja, entorno risk-on / complacencia.", vix))
		default:
			notes = append(notes, fmt.Sprintf("VIX en %.1f: volatilidad en rango normal.", vix))
		}
	}
	if mf := LatestMacroFile(cacheDir); mf != "" {
		df, err := quant.ReadCSVFile(mf)
		if err != nil {
			warn("No se pudo leer macro_data csv: %v", err)
		} else {
			find := func(sub string) (float64, bool) {
				ind, val := df.Col("Indicador"), df.Col("Último valor")
				if ind == nil || val == nil {
					return 0, false
				}
				for i := 0; i < df.N; i++ {
					if !ind.IsNull(i) && strings.Contains(ind.Cell(i), sub) {
						return val.At(i), true
					}
				}
				return 0, false
			}
			if spread, ok := find("Spread 10Y-2Y"); ok {
				if spread < 0 {
					notes = append(notes, fmt.Sprintf("Curva 10Y-2Y invertida (%.2f pp): señal histórica de precaución macro / riesgo de recesión.", spread))
				} else {
					notes = append(notes, fmt.Sprintf("Curva 10Y-2Y positiva (%.2f pp): sin señal de inversión.", spread))
				}
			}
			if hy, ok := find("Credit Spread High Yield"); ok {
				notes = append(notes, fmt.Sprintf("Spread de crédito High Yield: %.2f pp (referencia de apetito por riesgo).", hy))
			}
		}
	} else {
		notes = append(notes, "No se encontró macro_data_*.csv (los datos macro no se han descargado con FRED_API_KEY); contexto macro limitado a VIX/^IRX vía Yahoo.")
	}
	return notes
}

// readCloseCSV lee la columna Close de un CSV de precios.
func readCloseCSV(path string) ([]float64, error) {
	f, err := quant.ReadCSVFile(path)
	if err != nil {
		return nil, err
	}
	c := f.Col("Close")
	if c == nil {
		return nil, fmt.Errorf("%s no tiene columna Close", path)
	}
	return dropNaN(c.Floats()), nil
}

// LatestMacroFile devuelve el macro_data_<fecha>.csv más reciente de cache/macro.
func LatestMacroFile(cacheDir string) string {
	files, _ := filepath.Glob(filepath.Join(cacheDir, "macro", "macro_data_*.csv"))
	if len(files) == 0 {
		return ""
	}
	sort.Slice(files, func(i, j int) bool { return filepath.Base(files[i]) < filepath.Base(files[j]) })
	return files[len(files)-1]
}

// RunForecast escribe trend_forecast_6m<key>.csv de cada análisis cuya selección existe, y market_regime_notes.txt.
// Devuelve las notas.
func RunForecast(prices *Prices, runDir, cacheDir string, u *Universe, p Params, warn func(string, ...any)) ([]string, error) {
	for _, spec := range Analyses(u, p) {
		b, err := os.ReadFile(filepath.Join(runDir, "portfolio_performance"+spec.Key+".json"))
		if err != nil {
			if spec.Key == "" {
				return nil, fmt.Errorf("falta portfolio_performance.json: corre la selección de Markowitz antes de este paso")
			}
			continue // análisis complementario omitido por falta de tickers
		}
		var perf struct {
			Tickers []string `json:"tickers_seleccionados"`
		}
		if err := json.Unmarshal(b, &perf); err != nil {
			return nil, err
		}
		f := ForecastTable(prices, perf.Tickers, u, p)
		if err := writeFrameFile(filepath.Join(runDir, "trend_forecast_6m"+spec.Key+".csv"), f); err != nil {
			return nil, err
		}
		warn("Proyección a 6 meses (%s) generada para %s acciones.", spec.Label, strconv.Itoa(f.N))
	}
	notes := RegimeNotes(cacheDir, warn)
	return notes, os.WriteFile(filepath.Join(runDir, "market_regime_notes.txt"), []byte(strings.Join(notes, "\n")), 0o644)
}
