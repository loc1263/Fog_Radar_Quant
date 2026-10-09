package us

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

// Universo de acciones de EEUU a evaluar: yfinance no ofrece un «screener» nativo por market cap, así que
// se parte de la unión de los componentes del S&P 500 y el Nasdaq-100 (de Wikipedia), que cubre de sobra
// cualquier compañía estadounidense con más de 200B de capitalización. Si Wikipedia no responde, se usa la
// lista estática de respaldo.

const (
	sp500URL     = "https://en.wikipedia.org/wiki/List_of_S%26P_500_companies"
	nasdaq100URL = "https://en.wikipedia.org/wiki/Nasdaq-100"
	userAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
)

// CleanTicker normaliza al formato de Yahoo (BRK.B → BRK-B).
func CleanTicker(raw string) string {
	return strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(raw)), ".", "-")
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// ParseTickerTable busca la primera tabla HTML cuyo encabezado tenga alguna de las columnas candidatas y
// devuelve los tickers limpios de esa columna (equivale a pd.read_html + la columna).
func ParseTickerTable(html io.Reader, candidates ...string) (map[string]bool, error) {
	doc, err := goquery.NewDocumentFromReader(html)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	found := false
	doc.Find("table").EachWithBreak(func(_ int, tbl *goquery.Selection) bool {
		var headers []string
		tbl.Find("tr").First().Find("th").Each(func(_ int, th *goquery.Selection) {
			headers = append(headers, strings.TrimSpace(th.Text()))
		})
		col := -1
		for _, cand := range candidates {
			for i, h := range headers {
				if h == cand {
					col = i
					break
				}
			}
			if col >= 0 {
				break
			}
		}
		if col < 0 {
			return true // siguiente tabla
		}
		found = true
		tbl.Find("tr").Each(func(i int, tr *goquery.Selection) {
			if i == 0 {
				return
			}
			cells := tr.Find("td, th")
			if col >= cells.Length() {
				return
			}
			t := CleanTicker(cells.Eq(col).Text())
			if t != "" && isASCII(t) {
				out[t] = true
			}
		})
		return false
	})
	if !found {
		return map[string]bool{}, nil
	}
	return out, nil
}

func scrapeTable(ctx context.Context, url string, candidates ...string) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d en %s", resp.StatusCode, url)
	}
	return ParseTickerTable(resp.Body, candidates...)
}

// LargeCapUniverse devuelve los tickers candidatos (S&P 500 ∪ Nasdaq-100), ordenados.
func LargeCapUniverse(ctx context.Context, warn func(format string, args ...any)) []string {
	all := map[string]bool{}
	if t, err := scrapeTable(ctx, sp500URL, "Symbol"); err != nil {
		warn("No se pudo scrapear el S&P 500 de Wikipedia: %v", err)
	} else {
		for k := range t {
			all[k] = true
		}
	}
	if t, err := scrapeTable(ctx, nasdaq100URL, "Ticker", "Symbol"); err != nil {
		warn("No se pudo scrapear el Nasdaq-100 de Wikipedia: %v", err)
	} else {
		for k := range t {
			all[k] = true
		}
	}
	if len(all) == 0 {
		warn("Usando lista de respaldo estática de tickers")
		for _, t := range FallbackTickers {
			all[CleanTicker(t)] = true // el original dejaba «BRK.B», que Yahoo no reconoce (es BRK-B)
		}
	}
	out := make([]string, 0, len(all))
	for t := range all {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
