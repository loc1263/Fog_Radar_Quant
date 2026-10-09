package cl

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"

	"github.com/loc1263/gohoo"

	"fogradar/internal/fx"
	"fogradar/internal/quant"
	"fogradar/internal/yahoo"
)

//go:embed ipsa_tickers.csv
var ipsaTickersCSV []byte

// ColumnaTicker es la columna de ipsa_tickers.csv con el símbolo de Yahoo.
const ColumnaTicker = "Ticker (Yahoo Finance)"

// ClavesRetorno en el orden del CSV.
var ClavesRetorno = []string{"Retorno 5D", "Retorno 1M", "Retorno 3M", "Retorno 6M", "Retorno YTD", "Retorno 1A"}

// CargarTickers lee la lista de tickers del IPSA (sin repetidos, en orden).
func CargarTickers() ([]string, error) {
	df, err := quant.ReadCSV(strings.NewReader(string(ipsaTickersCSV)))
	if err != nil {
		return nil, err
	}
	c := df.Col(ColumnaTicker)
	if c == nil {
		return nil, fmt.Errorf("no se encontró la columna '%s'", ColumnaTicker)
	}
	seen := map[string]bool{}
	var out []string
	for i := 0; i < df.N; i++ {
		if c.IsNull(i) {
			continue
		}
		if t := c.Cell(i); !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out, nil
}

// Tipos de fundamentals-timeseries que usa el extractor.
const (
	TipoIngresos = "quarterlyTotalRevenue"
	TipoCapex    = "quarterlyCapitalExpenditure"
)

// CrecimientoIngresosYoY: ingresos del último trimestre vs. el mismo trimestre del año anterior
// (necesita ≥ 5 trimestres). pts viene del más reciente al más antiguo.
func CrecimientoIngresosYoY(pts []yahoo.Point) *float64 {
	if len(pts) < 5 {
		return nil
	}
	actual, antes := pts[0].Value, pts[4].Value
	if antes == 0 {
		return nil
	}
	v := (actual - antes) / math.Abs(antes)
	return &v
}

// LeveredFCF: Operating Cash Flow (ttm) + CapEx de los últimos 4 trimestres (el CapEx viene negativo).
func LeveredFCF(ocf *float64, capex []yahoo.Point) *float64 {
	if ocf == nil || len(capex) == 0 {
		return nil
	}
	sum := 0.0
	for i := 0; i < len(capex) && i < 4; i++ {
		sum += capex[i].Value
	}
	v := *ocf + sum
	return &v
}

// RetornosDesdePrecios calcula los retornos 5D/1M/3M/6M/YTD/1A (5/21/63/126/252 ruedas) desde cierres.
func RetornosDesdePrecios(bars []gohoo.Bar) map[string]*float64 {
	out := map[string]*float64{}
	for _, k := range ClavesRetorno {
		out[k] = nil
	}
	var closes []float64
	var years []int
	for _, b := range bars {
		if !math.IsNaN(b.Close) {
			closes = append(closes, b.Close)
			years = append(years, b.Time.Year())
		}
	}
	if len(closes) == 0 {
		return out
	}
	n := len(closes)
	actual := closes[n-1]
	anio := years[n-1]
	nDias := func(d int) *float64 {
		if n <= d || actual == 0 {
			return nil
		}
		base := closes[n-1-d]
		if base == 0 {
			return nil
		}
		v := actual/base - 1
		return &v
	}
	ytd := func() *float64 {
		if actual == 0 {
			return nil
		}
		for i := n - 1; i >= 0; i-- {
			if years[i] < anio {
				if closes[i] == 0 {
					return nil
				}
				v := actual/closes[i] - 1
				return &v
			}
		}
		return nil
	}
	out["Retorno 5D"], out["Retorno 1M"], out["Retorno 3M"] = nDias(5), nDias(21), nDias(63)
	out["Retorno 6M"], out["Retorno YTD"], out["Retorno 1A"] = nDias(126), ytd(), nDias(252)
	return out
}

func dividirSeguro(a, b *float64) *float64 {
	if a == nil || b == nil || *b == 0 {
		return nil
	}
	v := *a / *b
	return &v
}

func infoF(info map[string]any, k string) *float64 {
	if f, ok := gohoo.AsFloat(info[k]); ok && !math.IsNaN(f) {
		return &f
	}
	return nil
}

func infoS(info map[string]any, k string) *string {
	if s, ok := info[k].(string); ok {
		return &s
	}
	return nil
}

// Datos adicionales de un ticker que no vienen en info.
type Datos struct {
	Info      map[string]any
	Ingresos  []yahoo.Point
	Capex     []yahoo.Point
	Retornos  map[string]*float64
	TasaCambi func(origen, destino string) (float64, bool)
}

// Fila arma la fila de indicadores de un ticker (claves = columnas del CSV original), en orden.
func Fila(d Datos) []KV {
	info := d.Info
	monedaCot, monedaFin := infoS(info, "currency"), infoS(info, "financialCurrency")
	var tasa *float64
	one := 1.0
	tasa = &one
	if monedaCot != nil && monedaFin != nil && *monedaCot != "" && *monedaFin != "" && *monedaCot != *monedaFin {
		tasa = nil
		if r, ok := d.TasaCambi(*monedaFin, *monedaCot); ok {
			tasa = &r
		}
	}
	conv := func(v *float64) *float64 {
		if v == nil || tasa == nil {
			return nil
		}
		x := *v * *tasa
		return &x
	}

	ocfRaw := infoF(info, "operatingCashflow")
	leveredRaw := LeveredFCF(ocfRaw, d.Capex)
	revenue := conv(infoF(info, "totalRevenue"))
	ebitda := conv(infoF(info, "ebitda"))
	bookValuePS := conv(infoF(info, "bookValue"))
	marketCap, ev := infoF(info, "marketCap"), infoF(info, "enterpriseValue")
	precio := infoF(info, "currentPrice")
	if precio == nil || *precio == 0 { // 0 o ausente → cae al cálculo
		precio = dividirSeguro(marketCap, infoF(info, "sharesOutstanding"))
	}
	trailing := infoF(info, "trailingAnnualDividendYield")
	if trailing != nil {
		v := *trailing * 100
		trailing = &v
	}

	row := []KV{
		{"Símbolo", infoS(info, "symbol")}, {"Nombre corto", infoS(info, "shortName")},
		{"Sector", infoS(info, "sector")}, {"Industria", infoS(info, "industry")},
		{"Moneda Cotización", monedaCot}, {"Moneda Estados Financieros", monedaFin},
		{"Tasa Cambio Aplicada", tasa},
		{"Market Cap", marketCap}, {"Enterprise Value", ev},
		{"Trailing P/E", infoF(info, "trailingPE")}, {"Forward P/E", infoF(info, "forwardPE")},
		{"PEG Ratio (5yr expected)", infoF(info, "pegRatio")},
		{"Price/Sales", dividirSeguro(marketCap, revenue)}, {"Price/Book", dividirSeguro(precio, bookValuePS)},
		{"Enterprise Value/Revenue", dividirSeguro(ev, revenue)}, {"Enterprise Value/EBITDA", dividirSeguro(ev, ebitda)},
		{"Profit Margin", infoF(info, "profitMargins")}, {"Operating Margin (ttm)", infoF(info, "operatingMargins")},
		{"Return on Assets (ttm)", infoF(info, "returnOnAssets")}, {"Return on Equity (ttm)", infoF(info, "returnOnEquity")},
		{"Revenue (ttm)", revenue}, {"Revenue Per Share (ttm)", conv(infoF(info, "revenuePerShare"))},
		{"Quarterly Revenue Growth (yoy)", CrecimientoIngresosYoY(d.Ingresos)},
		{"Gross Profit (ttm)", conv(infoF(info, "grossProfits"))}, {"EBITDA", ebitda},
		{"Net Income Avi to Common (ttm)", conv(infoF(info, "netIncomeToCommon"))},
		{"Diluted EPS (ttm)", infoF(info, "trailingEps")},
		{"Quarterly Earnings Growth (yoy)", infoF(info, "earningsQuarterlyGrowth")},
		{"Total Cash (mrq)", conv(infoF(info, "totalCash"))}, {"Total Cash Per Share (mrq)", conv(infoF(info, "totalCashPerShare"))},
		{"Total Debt (mrq)", conv(infoF(info, "totalDebt"))}, {"Total Debt/Equity (mrq)", infoF(info, "debtToEquity")},
		{"Current Ratio (mrq)", infoF(info, "currentRatio")}, {"Book Value Per Share (mrq)", bookValuePS},
		{"Operating Cash Flow (ttm)", conv(ocfRaw)}, {"Levered Free Cash Flow (ttm)", conv(leveredRaw)},
		{"Beta (5Y Monthly)", infoF(info, "beta")}, {"52 Week Change", infoF(info, "52WeekChange")},
		{"S&P 500 52-Week Change", infoF(info, "SandP52WeekChange")},
		{"52 Week High", infoF(info, "fiftyTwoWeekHigh")}, {"52 Week Low", infoF(info, "fiftyTwoWeekLow")},
		{"50-Day Moving Average", infoF(info, "fiftyDayAverage")}, {"200-Day Moving Average", infoF(info, "twoHundredDayAverage")},
	}
	for _, k := range ClavesRetorno {
		row = append(row, KV{k, d.Retornos[k]})
	}
	row = append(row,
		KV{"Avg Vol (3 month)", infoF(info, "averageVolume")}, KV{"Avg Vol (10 day)", infoF(info, "averageVolume10days")},
		KV{"Shares Outstanding", infoF(info, "sharesOutstanding")}, KV{"Implied Shares Outstanding", infoF(info, "impliedSharesOutstanding")},
		KV{"Float", infoF(info, "floatShares")}, KV{"% Held by Insiders", infoF(info, "heldPercentInsiders")},
		KV{"% Held by Institutions", infoF(info, "heldPercentInstitutions")},
		KV{"Forward Annual Dividend Rate", infoF(info, "dividendRate")}, KV{"Forward Annual Dividend Yield (%)", infoF(info, "dividendYield")},
		KV{"Trailing Annual Dividend Rate", infoF(info, "trailingAnnualDividendRate")}, KV{"Trailing Annual Dividend Yield (%)", trailing},
		KV{"5 Year Average Dividend Yield (%)", infoF(info, "fiveYearAvgDividendYield")},
		KV{"Payout Ratio", infoF(info, "payoutRatio")}, KV{"Ex-Dividend Date", infoF(info, "exDividendDate")},
		KV{"Last Split Factor", infoS(info, "lastSplitFactor")}, KV{"Last Split Date", infoF(info, "lastSplitDate")},
	)
	return row
}

// KV es una celda de una fila: nombre de columna y valor (*float64, *string o nil).
type KV struct {
	K string
	V any
}

func unwrap(v any) any {
	switch x := v.(type) {
	case *float64:
		if x == nil {
			return nil
		}
		return *x
	case *string:
		if x == nil {
			return nil
		}
		return *x
	}
	return v
}

// ExtraerOpts configura la extracción.
type ExtraerOpts struct {
	Concurrencia int
	Log          *slog.Logger
}

// ExtraerIndicadores descarga los indicadores de todos los tickers (concurrencia acotada; los inicios de
// request siguen espaciados por el controlador de tráfico). Devuelve la tabla en el orden de los tickers y
// los que fallaron. Un ticker sin info se omite (queda en la lista de errores). No hay
// pausa fija cada 15 tickers (el espaciado global ya protege) y un símbolo inexistente no genera una fila vacía.
func ExtraerIndicadores(ctx context.Context, src yahoo.Source, tickers []string, o ExtraerOpts) (*quant.Frame, []string) {
	rates := fx.NewRates(fx.YahooPrice(src), func(f string, a ...any) { o.Log.Warn(fmt.Sprintf(f, a...)) })
	rowsOut := make([][]KV, len(tickers))
	ok := make([]bool, len(tickers))
	workers := max(1, min(o.Concurrencia, len(tickers)))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				t := tickers[i]
				o.Log.Info("indicadores", "ticker", t, "n", fmt.Sprintf("%d/%d", i+1, len(tickers)))
				info, err := yahoo.Info(ctx, src, t)
				if err != nil {
					o.Log.Error("no se pudo obtener el ticker, se omite", "ticker", t, "error", err)
					continue
				}
				d := Datos{Info: info, TasaCambi: func(a, b string) (float64, bool) { return rates.Rate(ctx, a, b) }}
				if fd, err := src.Fundamentals(ctx, t, []string{TipoIngresos, TipoCapex}); err != nil {
					o.Log.Warn("sin estados trimestrales", "ticker", t, "error", err)
				} else {
					d.Ingresos, d.Capex = yahoo.TimeseriesPoints(fd, TipoIngresos), yahoo.TimeseriesPoints(fd, TipoCapex)
				}
				if h, err := src.Chart(ctx, t, gohoo.ChartParams{Range: "2y", Interval: "1d"}); err != nil {
					o.Log.Warn("no se pudieron calcular los retornos de precio", "ticker", t, "error", err)
					d.Retornos = RetornosDesdePrecios(nil)
				} else {
					d.Retornos = RetornosDesdePrecios(yahoo.AutoAdjust(h.Bars))
				}
				rowsOut[i], ok[i] = Fila(d), true
			}
		}()
	}
	for i := range tickers {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	var rows [][]KV
	var errores []string
	for i, t := range tickers {
		if ok[i] {
			rows = append(rows, rowsOut[i])
		} else {
			errores = append(errores, t)
		}
	}
	return frameDe(rows), errores
}

func frameDe(rows [][]KV) *quant.Frame {
	f := quant.NewFrame(len(rows))
	if len(rows) == 0 {
		return f
	}
	for j, kv := range rows[0] {
		vals := make([]any, len(rows))
		for i := range rows {
			vals[i] = unwrap(rows[i][j].V)
		}
		c := quant.Cells(kv.K, vals)
		f.Set(c)
	}
	return f
}
