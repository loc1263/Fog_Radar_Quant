// Busca en qué módulos de quoteSummary aparece cada campo. Uso: go run ./spikes/info/find TICKER campo...
package main

import (
	"context"
	"fmt"
	"os"

	_ "time/tzdata"

	"github.com/loc1263/gohoo"
)

func main() {
	c, err := gohoo.NewClient()
	if err != nil {
		panic(err)
	}
	mods := []string{"price", "summaryDetail", "defaultKeyStatistics", "financialData", "assetProfile", "quoteType", "fundProfile", "topHoldings", "fundPerformance", "summaryProfile", "calendarEvents"}
	for _, m := range mods {
		qs, err := c.QuoteSummaryContext(context.Background(), os.Args[1], []string{m})
		if err != nil {
			fmt.Println(m, "error:", err)
			continue
		}
		mm, _ := qs[m].(map[string]any)
		for _, f := range os.Args[2:] {
			if v, ok := mm[f]; ok {
				fmt.Printf("%-22s %-24s %v\n", m, f, gohoo.RawValue(v))
			}
		}
		if len(os.Args) == 2 {
			fmt.Println(m, len(mm), "claves")
		}
	}
}
