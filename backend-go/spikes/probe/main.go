package main

import (
	"encoding/json"
	"fmt"

	_ "time/tzdata"

	"github.com/loc1263/gohoo"
)

func main() {
	c, _ := gohoo.NewClient()
	for _, tk := range []string{"SPY", "VTI", "IBIT"} {
		qs, err := c.QuoteSummary(tk, []string{"fundProfile", "price", "summaryDetail", "defaultKeyStatistics", "quoteType"})
		if err != nil {
			fmt.Println(err)
			continue
		}
		b, _ := json.Marshal(gohoo.GetPath(qs, "fundProfile", "feesExpensesInvestment"))
		fmt.Println(tk, string(b))
		for _, p := range [][]string{{"price", "regularMarketPrice"}, {"price", "quoteType"}, {"price", "longName"}, {"summaryDetail", "volume"}, {"summaryDetail", "totalAssets"}, {"defaultKeyStatistics", "totalAssets"}, {"defaultKeyStatistics", "fundInceptionDate"}, {"fundProfile", "categoryName"}, {"summaryDetail", "fiftyTwoWeekHigh"}, {"quoteType", "quoteType"}, {"summaryDetail", "regularMarketVolume"}, {"price", "regularMarketVolume"}} {
			fmt.Printf("   %v = %v\n", p, gohoo.RawValue(gohoo.GetPath(qs, p...)))
		}
	}
}
