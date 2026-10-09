// Spike A2 (Fase 0): throughput de gohoo bajo espaciado global de INICIOS (no de término) y N workers.
// Mide si la concurrencia acotada reduce el tiempo sin provocar 429. Uso: go run . <workers> <intervalo_ms> <n>
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "time/tzdata"

	"github.com/loc1263/gohoo"
)

var base = []string{"AAPL", "MSFT", "NVDA", "AMZN", "GOOGL", "META", "TSLA", "JPM", "XOM", "JNJ", "SPY", "QQQ", "VTI", "EEM", "GLD",
	"TLT", "VWO", "IWM", "XLK", "ARKK", "KO", "PEP", "WMT", "HD", "BAC", "WFC", "CVX", "PFE", "MRK", "ABBV", "COST", "ORCL", "CSCO", "INTC", "AMD"}

func main() {
	workers, _ := strconv.Atoi(os.Args[1])
	ms, _ := strconv.Atoi(os.Args[2])
	n, _ := strconv.Atoi(os.Args[3])
	c, err := gohoo.NewClientWithOptions(gohoo.WithRetryPolicy(gohoo.RetryPolicy{MaxAttempts: 1}))
	if err != nil {
		panic(err)
	}
	var gate sync.Mutex
	next := time.Now()
	wait := func() { // espaciador global de inicios
		gate.Lock()
		d := time.Until(next)
		next = maxT(next, time.Now()).Add(time.Duration(ms) * time.Millisecond)
		gate.Unlock()
		if d > 0 {
			time.Sleep(d)
		}
	}
	var ok, r429, other atomic.Int64
	jobs := make(chan string)
	var wg sync.WaitGroup
	t0 := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for tk := range jobs {
				wait()
				var err error
				if len(os.Args) > 4 { // modo pipeline: quoteSummary + chart 1y por ticker (2 requests)
					_, err = c.QuoteSummaryContext(context.Background(), tk, []string{"price", "summaryDetail", "defaultKeyStatistics"})
					if err == nil || !errors.Is(err, gohoo.ErrRateLimit) {
						wait()
						_, err2 := c.ChartRange(context.Background(), tk, gohoo.ChartParams{Range: "1y", Interval: "1d"})
						if err == nil {
							err = err2
						}
					}
				} else {
					_, err = c.ChartRange(context.Background(), tk, gohoo.ChartParams{Range: "5y", Interval: "1wk"})
				}
				switch {
				case err == nil:
					ok.Add(1)
				case errors.Is(err, gohoo.ErrRateLimit):
					r429.Add(1)
				default:
					if other.Add(1) <= 3 {
						fmt.Println("error de muestra:", tk, err)
					}
				}
			}
		}()
	}
	list := base
	if len(os.Args) > 4 { // lista de tickers (una por línea) para un volumen realista
		if b, err := os.ReadFile(os.Args[4]); err == nil {
			list = strings.Fields(string(b))
		}
	}
	for i := 0; i < n; i++ {
		jobs <- list[i%len(list)]
	}
	close(jobs)
	wg.Wait()
	el := time.Since(t0)
	fmt.Printf("workers=%d intervalo=%dms n=%d -> %v (%.2f req/s) ok=%d 429=%d otros=%d\n", workers, ms, n, el.Round(time.Millisecond), float64(n)/el.Seconds(), ok.Load(), r429.Load(), other.Load())
}

func maxT(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
