package tend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/loc1263/gohoo"

	"fogradar/internal/quant"
	"fogradar/internal/yahoo"
)

// Datos macro de referencia (macro.py): curva de tasas, spreads de crédito, VIX, proxies de PMI y postura de la Fed.
// Salida en data/tendencia/macro/macro_data_<YYYYMMDD>.csv y fomc_latest_statement.txt.

type fredSerie struct{ Label, ID string }

// Series principales de FRED (en el orden del CSV).
var fredSeries = []fredSerie{
	{"Treasury 3M (%)", "DGS3MO"}, {"Treasury 2Y (%)", "DGS2"}, {"Treasury 10Y (%)", "DGS10"}, {"Treasury 30Y (%)", "DGS30"},
	{"Spread 10Y-2Y (pp)", "T10Y2Y"}, {"Spread 10Y-3M (pp)", "T10Y3M"},
	{"Credit Spread High Yield (pp)", "BAMLH0A0HYM2"}, {"Credit Spread Investment Grade (pp)", "BAMLC0A0CM"},
	{"Credit Spread BBB (pp)", "BAMLC0A4CBBB"}, {"VIX (FRED)", "VIXCLS"},
	{"Fed Funds Rate efectiva (%)", "FEDFUNDS"}, {"Fed Funds Target Upper (%)", "DFEDTARU"},
	{"Fed Funds Target Lower (%)", "DFEDTARL"}, {"Fed Dot Plot mediana (%)", "FEDTARMD"},
	{"Chicago Fed National Activity Index (CFNAI)", "CFNAI"},
}

// Proxies regionales de PMI: best-effort (algunos IDs pueden no existir).
var fredBestEffort = []fredSerie{
	{"Philly Fed Manufacturing Index", "GACDISA066MSFRBPHI"}, {"NY Fed Empire State Manufacturing Index", "GACDFSA066MSFRBNY"},
	{"Richmond Fed Manufacturing Index", "RSAFSMFTIX"}, {"Dallas Fed Manufacturing Index", "BACTSAMFRBDAL"},
	{"Kansas City Fed Manufacturing Index", "MMSMFGCOMP"},
}

// MacroOpts son las dependencias de la descarga macro (inyectables en los tests).
type MacroOpts struct {
	APIKey  string
	HTTP    *http.Client
	FredURL string // por defecto https://api.stlouisfed.org/fred
	FedURL  string // por defecto https://www.federalreserve.gov
	Yahoo   yahoo.Source
	Now     time.Time
	Log     func(string, ...any)
}

type macroRow struct {
	Indicador, SeriesID string
	Valor               float64
	Fecha               string
}

// lastObservation devuelve el último valor no faltante de una serie de FRED.
func lastObservation(ctx context.Context, o MacroOpts, id string) (float64, string, error) {
	q := url.Values{"series_id": {id}, "api_key": {o.APIKey}, "file_type": {"json"}, "sort_order": {"desc"}, "limit": {"20"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.FredURL+"/series/observations?"+q.Encode(), nil)
	if err != nil {
		return 0, "", err
	}
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var out struct {
		Observations []struct {
			Date  string `json:"date"`
			Value string `json:"value"`
		} `json:"observations"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, "", err
	}
	for _, ob := range out.Observations { // en orden descendente: la primera válida es la última
		if ob.Value == "." || ob.Value == "" {
			continue
		}
		var v float64
		if _, err := fmt.Sscanf(ob.Value, "%g", &v); err != nil {
			continue
		}
		return v, ob.Date, nil
	}
	return 0, "", errors.New("serie vacía")
}

func fetchFred(ctx context.Context, o MacroOpts, series []fredSerie, required bool) []macroRow {
	var rows []macroRow
	for _, s := range series {
		v, d, err := lastObservation(ctx, o, s.ID)
		if err != nil {
			status := "ERROR"
			if !required {
				status = "omitido (best-effort)"
			}
			o.Log("  [%s] %s (%s): %v", status, s.Label, s.ID, err)
			continue
		}
		rows = append(rows, macroRow{s.Label, s.ID, v, d})
	}
	return rows
}

var fomcLink = regexp.MustCompile(`/newsevents/pressreleases/monetary\d{8}a\.htm$`)

// fetchFOMC busca el comunicado FOMC más reciente (best-effort) y lo guarda en dir.
func fetchFOMC(ctx context.Context, o MacroOpts, dir string) {
	get := func(u string) (*goquery.Document, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := o.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return goquery.NewDocumentFromReader(resp.Body)
	}
	index := fmt.Sprintf("%s/newsevents/pressreleases/%d-press-fomc.htm", o.FedURL, o.Now.Year())
	doc, err := get(index)
	if err != nil {
		o.Log("  [ERROR] No se pudo obtener el comunicado FOMC: %v", err)
		return
	}
	set := map[string]bool{}
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		if h, _ := a.Attr("href"); fomcLink.MatchString(h) {
			set[h] = true
		}
	})
	var cands []string
	for h := range set {
		cands = append(cands, h)
	}
	if len(cands) == 0 {
		o.Log("  [ERROR] No se pudo obtener el comunicado FOMC: no se encontró un enlace de comunicado monetario en %s", index)
		return
	}
	sort.Sort(sort.Reverse(sort.StringSlice(cands)))
	full := o.FedURL + cands[0]
	sdoc, err := get(full)
	if err != nil {
		o.Log("  [ERROR] No se pudo obtener el comunicado FOMC: %v", err)
		return
	}
	sel := sdoc.Find("div#article")
	if sel.Length() == 0 {
		sel = sdoc.Find("div.col-xs-12.col-sm-8.col-md-8")
	}
	if sel.Length() == 0 {
		sel = sdoc.Selection
	}
	var parts []string
	sel.Find("*").Each(func(_ int, n *goquery.Selection) {
		if n.Children().Length() == 0 {
			if t := strings.TrimSpace(n.Text()); t != "" {
				parts = append(parts, t)
			}
		}
	})
	text := strings.Join(parts, "\n")
	out := filepath.Join(dir, "fomc_latest_statement.txt")
	if err := os.WriteFile(out, []byte(fmt.Sprintf("Fuente: %s\nDescargado: %s\n\n%s", full, o.Now.Format("2006-01-02T15:04:05.000000"), text)), 0o644); err != nil {
		o.Log("  [ERROR] No se pudo guardar el comunicado FOMC: %v", err)
		return
	}
	o.Log("  Comunicado FOMC guardado en: %s", out)
}

func fetchVIX(ctx context.Context, o MacroOpts) *macroRow {
	if o.Yahoo == nil {
		return nil
	}
	res, err := o.Yahoo.Chart(ctx, "^VIX", gohoo.ChartParams{Range: "5d", Interval: "1d"})
	if err != nil || len(res.Bars) == 0 {
		o.Log("  [ERROR] VIX vía yfinance: %v", err)
		return nil
	}
	b := res.Bars[len(res.Bars)-1]
	return &macroRow{"VIX (Yahoo Finance, spot)", "^VIX", b.Close, b.Time.Format("2006-01-02")}
}

// FetchMacro descarga los datos macro y devuelve la ruta del CSV (macro_data_<fecha>.csv en dir).
func FetchMacro(ctx context.Context, dir string, o MacroOpts) (string, error) {
	if o.APIKey == "" {
		return "", errors.New("no se encontró FRED_API_KEY (consigue una key gratis en https://fred.stlouisfed.org/docs/api/api_key.html y ponla en .env)")
	}
	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: 15 * time.Second}
	}
	if o.FredURL == "" {
		o.FredURL = "https://api.stlouisfed.org/fred"
	}
	if o.FedURL == "" {
		o.FedURL = "https://www.federalreserve.gov"
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	if o.Log == nil {
		o.Log = func(string, ...any) {}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	o.Log("[FRED] series principales (curva, spreads, VIX, Fed, CFNAI)...")
	rows := fetchFred(ctx, o, fredSeries, true)
	o.Log("[FRED] proxies regionales de PMI (best-effort)...")
	rows = append(rows, fetchFred(ctx, o, fredBestEffort, false)...)
	o.Log("[Yahoo Finance] VIX spot...")
	if r := fetchVIX(ctx, o); r != nil {
		rows = append(rows, *r)
	}
	o.Log("[federalreserve.gov] comunicado FOMC más reciente...")
	fetchFOMC(ctx, o, dir)
	if len(rows) == 0 {
		return "", errors.New("no se pudo obtener ningún dato macro: revisa FRED_API_KEY y la conexión")
	}
	n := len(rows)
	ind, sid, val, fec := make([]string, n), make([]string, n), make([]float64, n), make([]string, n)
	for i, r := range rows {
		ind[i], sid[i], val[i], fec[i] = r.Indicador, r.SeriesID, r.Valor, r.Fecha
	}
	f := quant.NewFrame(n)
	f.Set(strCol("Indicador", ind))
	f.Set(strCol("Series ID", sid))
	f.Set(fCol("Último valor", val))
	f.Set(strCol("Fecha dato", fec))
	out := filepath.Join(dir, "macro_data_"+o.Now.Format("20060102")+".csv")
	if err := writeFrameFile(out, f); err != nil {
		return "", err
	}
	o.Log("Guardado en: %s (%d series)", out, n)
	return out, nil
}
