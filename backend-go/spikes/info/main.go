// Spike (Fase 2): compara yahoo.FlattenInfo con Ticker.info de yfinance. Uso: go run ./spikes/info <out_py.json>
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"

	_ "time/tzdata"

	"fogradar/internal/core"
	"fogradar/internal/yahoo"
)

var fields = []string{"shortName", "longName", "sector", "industry", "country", "currency", "financialCurrency", "quoteType", "symbol",
	"currentPrice", "regularMarketPrice", "previousClose", "marketCap", "enterpriseValue", "trailingPE", "forwardPE", "pegRatio",
	"trailingPegRatio", "priceToBook", "enterpriseToEbitda", "profitMargins", "operatingMargins", "returnOnAssets", "returnOnEquity",
	"earningsQuarterlyGrowth", "totalRevenue", "revenuePerShare", "grossProfits", "ebitda", "netIncomeToCommon", "trailingEps",
	"forwardEps", "operatingCashflow", "freeCashflow", "totalCash", "totalCashPerShare", "totalDebt", "debtToEquity", "currentRatio",
	"bookValue", "beta", "52WeekChange", "SandP52WeekChange", "fiftyTwoWeekHigh", "fiftyTwoWeekLow", "fiftyDayAverage",
	"twoHundredDayAverage", "averageVolume", "averageVolume10days", "volume", "sharesOutstanding", "impliedSharesOutstanding",
	"floatShares", "heldPercentInsiders", "heldPercentInstitutions", "dividendRate", "dividendYield", "trailingAnnualDividendRate",
	"trailingAnnualDividendYield", "fiveYearAvgDividendYield", "payoutRatio", "exDividendDate", "lastSplitFactor", "lastSplitDate",
	"category", "totalAssets", "fundInceptionDate", "netExpenseRatio", "annualReportExpenseRatio"}

func main() {
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var py map[string]map[string]any
	if err := json.Unmarshal(raw, &py); err != nil {
		panic(err)
	}
	s, _ := core.LoadSettings()
	tr := yahoo.NewTraffic(yahoo.PolicyFromSettings(s))
	c := yahoo.NewClient(tr)
	tickers := make([]string, 0, len(py))
	for t := range py {
		tickers = append(tickers, t)
	}
	sort.Strings(tickers)
	diff := map[string]int{}
	for _, t := range tickers {
		info, err := yahoo.Info(context.Background(), c, t)
		if err != nil {
			fmt.Println(t, "ERROR", err)
			continue
		}
		for _, f := range fields {
			a, aok := py[t][f]
			b, bok := info[f]
			if !aok && !bok {
				continue
			}
			if aok && bok && same(a, b) {
				continue
			}
			diff[f]++
			fmt.Printf("%-10s %-28s py=%v go=%v\n", t, f, trunc(a, aok), trunc(b, bok))
		}
	}
	fmt.Println("\nresumen de diferencias por campo:", diff)
}

func trunc(v any, ok bool) any {
	if !ok {
		return "—"
	}
	return v
}

func same(a, b any) bool {
	fa, ok1 := a.(float64)
	fb, ok2 := b.(float64)
	if ok1 && ok2 {
		return math.Abs(fa-fb) <= 1e-9*math.Max(1, math.Abs(fa))
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}
