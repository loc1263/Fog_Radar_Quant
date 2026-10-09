// Spike A+B (Fase 0): ¿gohoo (net/http) obtiene datos de Yahoo con el espaciado actual,
// y qué tan equivalentes son frente a yfinance? Salida: out_go.json + resumen en consola.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"time"

	_ "time/tzdata"

	"github.com/loc1263/gohoo"
)

var tickers = []string{
	"AAPL", "MSFT", "NVDA", "AMZN", "GOOGL", "META", "TSLA", "JPM", "XOM", "JNJ",
	"SPY", "QQQ", "VTI", "EEM", "GLD", "TLT", "VWO", "IWM", "XLK", "ARKK",
	"SQM-B.SN", "FALABELLA.SN", "COPEC.SN", "CHILE.SN", "BSANTANDER.SN", "CENCOSUD.SN", "ENELAM.SN", "CMPC.SN",
	"ASML", "SAP", "7203.T", "0700.HK", "VOD.L", "BHP.AX", "NESN.SW",
	"CLP=X", "EURUSD=X", "GBPUSD=X", "JPY=X", "^VIX", "^GSPC", "BTC-USD",
}

type result struct {
	Ticker   string             `json:"ticker"`
	Bars     int                `json:"bars"`
	LastDate string             `json:"last_date"`
	LastClose float64           `json:"last_close"`
	Info     map[string]any     `json:"info,omitempty"`
	ErrChart string             `json:"err_chart,omitempty"`
	ErrInfo  string             `json:"err_info,omitempty"`
	MsChart  int64              `json:"ms_chart"`
	MsInfo   int64              `json:"ms_info"`
}

func classify(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, gohoo.ErrRateLimit):
		return "429"
	case errors.Is(err, gohoo.ErrUnauthorized):
		return "401/403"
	case errors.Is(err, gohoo.ErrNotFound):
		return "404"
	}
	return "otro"
}

func main() {
	minInt, jit := 350*time.Millisecond, 250*time.Millisecond // = settings.py
	t0 := time.Now()
	c, err := gohoo.NewClient()
	if err != nil {
		fmt.Println("NewClient FALLÓ (cookie/crumb):", err)
		os.Exit(1)
	}
	fmt.Printf("bootstrap cookie+crumb OK en %v\n", time.Since(t0).Round(time.Millisecond))

	stats := map[string]int{}
	var out []result
	ctx := context.Background()
	for _, tk := range tickers {
		r := result{Ticker: tk}
		time.Sleep(minInt + time.Duration(rand.Float64()*float64(jit)))
		s := time.Now()
		res, err := c.ChartRange(ctx, tk, gohoo.ChartParams{Range: "1y", Interval: "1d"})
		r.MsChart = time.Since(s).Milliseconds()
		stats["chart_"+classify(err)]++
		if err != nil {
			r.ErrChart = err.Error()
		} else if len(res.Bars) > 0 {
			b := res.Bars[len(res.Bars)-1]
			r.Bars, r.LastDate, r.LastClose = len(res.Bars), b.Time.Format("2006-01-02"), b.Close
		}
		time.Sleep(minInt + time.Duration(rand.Float64()*float64(jit)))
		s = time.Now()
		qs, err := c.QuoteSummaryContext(ctx, tk, []string{"price", "summaryDetail", "defaultKeyStatistics", "assetProfile"})
		r.MsInfo = time.Since(s).Milliseconds()
		stats["info_"+classify(err)]++
		if err != nil {
			r.ErrInfo = err.Error()
		} else {
			r.Info = qs
		}
		out = append(out, r)
		fmt.Printf("%-14s chart=%-4d %s %8.2f (%dms) info=%v (%dms)\n", tk, r.Bars, r.LastDate, r.LastClose, r.MsChart, r.ErrInfo == "", r.MsInfo)
	}
	fmt.Printf("\nTOTAL %v · %d tickers\n%v\n", time.Since(t0).Round(time.Second), len(tickers), stats)
	f, _ := os.Create("out_go.json")
	json.NewEncoder(f).Encode(out)
	f.Close()
}
