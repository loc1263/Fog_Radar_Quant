// Spike B2 (Fase 0): FundamentalsTimeseries vs quarterly_income_stmt/quarterly_cashflow (extractor de Chile).
// Uso: go run . ticker1 ticker2 ...   Salida: out_fund.json {ticker:{type:[[fecha,valor],...] (más reciente primero)}}
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	_ "time/tzdata"

	"github.com/loc1263/gohoo"
)

var types = []string{"quarterlyTotalRevenue", "quarterlyCapitalExpenditure"}

func main() {
	c, err := gohoo.NewClient()
	if err != nil {
		panic(err)
	}
	out := map[string]map[string][][2]any{}
	for _, tk := range os.Args[1:] {
		time.Sleep(500 * time.Millisecond)
		data, err := c.FundamentalsTimeseries(tk, types)
		if err != nil {
			fmt.Println(tk, "ERROR", err)
			continue
		}
		res := out[tk]
		if res == nil {
			res = map[string][][2]any{}
			out[tk] = res
		}
		ts, _ := gohoo.GetPath(data, "timeseries", "result").([]any)
		for _, it := range ts {
			m, _ := it.(map[string]any)
			for _, t := range types {
				arr, ok := m[t].([]any)
				if !ok {
					continue
				}
				for _, e := range arr {
					em, _ := e.(map[string]any)
					if em == nil {
						continue
					}
					fecha, _ := em["asOfDate"].(string)
					var v any
					if rv, ok := em["reportedValue"].(map[string]any); ok {
						v = rv["raw"]
					}
					res[t] = append(res[t], [2]any{fecha, v})
				}
			}
		}
		for t := range res {
			sort.Slice(res[t], func(i, j int) bool { return res[t][i][0].(string) > res[t][j][0].(string) })
		}
		fmt.Printf("%-14s rev=%d capex=%d\n", tk, len(res["quarterlyTotalRevenue"]), len(res["quarterlyCapitalExpenditure"]))
	}
	f, _ := os.Create("out_fund.json")
	json.NewEncoder(f).Encode(out)
	f.Close()
}
