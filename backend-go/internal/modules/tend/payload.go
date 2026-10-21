package tend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"fogradar/internal/quant"
)

// Payload del dashboard: une, por análisis, los archivos que dejaron
// Markowitz, la proyección y el momentum. Se arma como OM para conservar el orden de claves de dashboard_data.json.

// ParseOM decodifica JSON conservando el orden de las claves (objetos → OM, enteros → int64, números → float64).
func ParseOM(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	return parseValue(dec)
}

func parseValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		if t == '{' {
			om := OM{}
			for dec.More() {
				k, err := dec.Token()
				if err != nil {
					return nil, err
				}
				v, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				om = append(om, KV{k.(string), v})
			}
			_, err := dec.Token()
			return om, err
		}
		arr := []any{}
		for dec.More() {
			v, err := parseValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		_, err := dec.Token()
		return arr, err
	case json.Number:
		s := t.String()
		if !strings.ContainsAny(s, ".eE") {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				return n, nil
			}
		}
		f, err := t.Float64()
		return f, err
	}
	return tok, nil // string, bool, nil
}

// Get devuelve el valor de una clave de un OM (nil si no está).
func (o OM) Get(k string) any {
	for _, kv := range o {
		if kv.K == k {
			return kv.V
		}
	}
	return nil
}

// round10 es el redondeo a 10 decimales de DataFrame.to_json (double_precision=10).
func round10(x float64) float64 {
	v, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 10, 64), 64)
	return v
}

func jsonVal(c *quant.Column, i int) any {
	v := c.Value(i)
	if f, ok := v.(float64); ok {
		return round10(f)
	}
	return v
}

// records convierte un Frame a registros (como DataFrame.to_json(orient="records")). Un frame nil/vacío da [].
func records(f *quant.Frame) []any {
	out := []any{}
	if f == nil {
		return out
	}
	for i := 0; i < f.N; i++ {
		row := OM{}
		for _, c := range f.Cols {
			row = append(row, KV{c.Name, jsonVal(c, i)})
		}
		out = append(out, row)
	}
	return out
}

// readCSVOrEmpty lee un CSV; nil si no existe, está vacío o no tiene datos.
func readCSVOrEmpty(path string) *quant.Frame {
	b, err := os.ReadFile(path)
	if err != nil || len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	f, err := quant.ReadCSVFile(path)
	if err != nil {
		return nil
	}
	return f
}

type holding struct {
	ticker string
	peso   float64
	cells  OM // todas las claves en orden, incluido ticker
}

// mergeHoldings une los pesos finales con las columnas de la proyección (left join por ticker) y agrega el sector.
// Orden: peso_final descendente (estable).
func mergeHoldings(weights, forecast *quant.Frame, u *Universe) []holding {
	var fcols []*quant.Column
	fidx := map[string]int{}
	if forecast != nil {
		tk := forecast.Col("ticker")
		for i := 0; i < forecast.N; i++ {
			fidx[tk.Cell(i)] = i
		}
		for _, c := range forecast.Cols {
			switch c.Name {
			case "ticker", "nombre", "pais", "sector":
			default:
				fcols = append(fcols, c)
			}
		}
	}
	idxCol := weights.Cols[0]
	var out []holding
	for i := 0; i < weights.N; i++ {
		t := idxCol.Cell(i)
		cells := OM{{"ticker", t}}
		for _, c := range weights.Cols[1:] {
			cells = append(cells, KV{c.Name, jsonVal(c, i)})
		}
		fi, ok := fidx[t]
		for _, c := range fcols {
			if ok {
				cells = append(cells, KV{c.Name, jsonVal(c, fi)})
			} else {
				cells = append(cells, KV{c.Name, nil})
			}
		}
		cells = append(cells, KV{"sector", u.SectorOf(t)})
		w := math.NaN()
		if pc := weights.Col("peso_final"); pc != nil {
			w = pc.At(i)
		}
		out = append(out, holding{t, w, cells})
	}
	sort.SliceStable(out, func(a, b int) bool {
		x, y := out[a].peso, out[b].peso
		if math.IsNaN(x) {
			return false
		}
		return math.IsNaN(y) || x > y
	})
	return out
}

func holdingRecords(h []holding) []any {
	out := make([]any, len(h))
	for i, x := range h {
		out[i] = x.cells
	}
	return out
}

func kahanSum(a []float64) float64 {
	s, c := 0.0, 0.0
	for _, v := range a {
		y := v - c
		t := s + y
		c = (t - s) - y
		s = t
	}
	return s
}

// sectorSummary reproduce compute_sector_summary: por sector, n, peso total, retorno 6m promedio, señal predominante y
// tickers; ordenado por retorno_6m_prom descendente (NaN al final).
func sectorSummary(h []holding) []any {
	type grp struct {
		pesos   []float64
		rets    []float64
		signals []string
		tickers []string
	}
	by := map[string]*grp{}
	var names []string
	for _, x := range h {
		sec := x.cells.Get("sector").(string)
		g, ok := by[sec]
		if !ok {
			g = &grp{}
			by[sec] = g
			names = append(names, sec)
		}
		if !math.IsNaN(x.peso) {
			g.pesos = append(g.pesos, x.peso)
		}
		if r, ok := x.cells.Get("retorno_esperado_6m_pct").(float64); ok {
			g.rets = append(g.rets, r)
		}
		if s, ok := x.cells.Get("senal_tecnica").(string); ok {
			g.signals = append(g.signals, s)
		}
		g.tickers = append(g.tickers, x.ticker)
	}
	sort.Strings(names)
	type row struct {
		cells OM
		ret   float64
	}
	var rows []row
	for _, n := range names {
		g := by[n]
		ret := math.NaN()
		var retV any
		if len(g.rets) > 0 {
			ret = quant.KahanMean(g.rets)
			retV = round10(ret)
		}
		var sig any
		counts := map[string]int{}
		best, bestN := "", 0
		for _, s := range g.signals {
			counts[s]++
		}
		for _, s := range g.signals { // idxmax del value_counts: el primero con el máximo, en orden de aparición
			if counts[s] > bestN {
				best, bestN = s, counts[s]
			}
		}
		if bestN > 0 {
			sig = best
		}
		rows = append(rows, row{OM{{"sector", n}, {"n_acciones", int64(len(g.pesos))}, {"peso_total", round10(kahanSum(g.pesos))},
			{"retorno_6m_prom", retV}, {"senal_predominante", sig}, {"tickers", strings.Join(g.tickers, ", ")}}, ret})
	}
	sort.SliceStable(rows, func(a, b int) bool {
		x, y := rows[a].ret, rows[b].ret
		if math.IsNaN(x) {
			return false
		}
		return math.IsNaN(y) || x > y
	})
	out := make([]any, len(rows))
	for i, r := range rows {
		out[i] = r.cells
	}
	return out
}

func readNotes(runDir string) []any {
	out := []any{}
	b, err := os.ReadFile(filepath.Join(runDir, "market_regime_notes.txt"))
	if err != nil {
		return out
	}
	for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// AnalysisPayload es el payload JSON de un análisis. (nil, nil) si el análisis complementario se omitió.
func AnalysisPayload(runDir string, spec Spec, u *Universe, p Params) (OM, error) {
	perfPath := filepath.Join(runDir, "portfolio_performance"+spec.Key+".json")
	pb, err := os.ReadFile(perfPath)
	if err != nil {
		if spec.Key != "" {
			return nil, nil
		}
		return nil, fmt.Errorf("falta %s", filepath.Base(perfPath))
	}
	perfAny, err := ParseOM(pb)
	if err != nil {
		return nil, err
	}
	perf := perfAny.(OM)
	weights, err := quant.ReadCSVFile(filepath.Join(runDir, "portfolio_final_weights"+spec.Key+".csv"))
	if err != nil {
		return nil, err
	}
	forecast := readCSVOrEmpty(filepath.Join(runDir, "trend_forecast_6m"+spec.Key+".csv"))
	merged := mergeHoldings(weights, forecast, u)

	nSel := len(merged)
	if ts, ok := perf.Get("tickers_seleccionados").([]any); ok {
		nSel = len(ts)
	}
	options := []any{}
	for _, n := range p.PortfolioSizes {
		wp := filepath.Join(runDir, fmt.Sprintf("portfolio_final_weights_top%d%s.csv", n, spec.Key))
		pp := filepath.Join(runDir, fmt.Sprintf("portfolio_performance_top%d%s.json", n, spec.Key))
		if !isFile(wp) || !isFile(pp) {
			continue
		}
		rw, err := quant.ReadCSVFile(wp)
		if err != nil {
			return nil, err
		}
		rb, err := os.ReadFile(pp)
		if err != nil {
			return nil, err
		}
		rperf, err := ParseOM(rb)
		if err != nil {
			return nil, err
		}
		options = append(options, OM{{"n", int64(n)}, {"performance", rperf}, {"holdings", holdingRecords(mergeHoldings(rw, forecast, u))}})
	}
	return OM{
		{"key", spec.Key}, {"label", spec.Label}, {"universe_size", int64(len(spec.Tickers))}, {"n_selected", int64(nSel)},
		{"performance", perf}, {"holdings", holdingRecords(merged)}, {"sectors", sectorSummary(merged)},
		{"momentum", records(readCSVOrEmpty(filepath.Join(runDir, "top_momentum_mensual"+spec.Key+".csv")))},
		{"momentum_semanal", records(readCSVOrEmpty(filepath.Join(runDir, "top_momentum_semanal"+spec.Key+".csv")))},
		{"retorno_reciente", records(readCSVOrEmpty(filepath.Join(runDir, "top_retorno_reciente"+spec.Key+".csv")))},
		{"portfolio_options", options},
	}, nil
}

// BuildDashboardPayload arma el payload completo de una corrida.
func BuildDashboardPayload(runDir string, u *Universe, p Params, now time.Time) (OM, error) {
	var analyses []any
	for _, spec := range Analyses(u, p) {
		a, err := AnalysisPayload(runDir, spec, u, p)
		if err != nil {
			return nil, err
		}
		if a != nil {
			analyses = append(analyses, a)
		}
	}
	if len(analyses) == 0 {
		return nil, fmt.Errorf("no se generó ningún análisis (ver Markowitz); nada que exportar")
	}
	return OM{{"generated_at", now.Format("2006-01-02T15:04:05.000000")}, {"run_dir", filepath.Base(runDir)},
		{"universe_source", u.Source}, {"market_regime_notes", readNotes(runDir)}, {"analyses", analyses}}, nil
}

// WriteDashboard escribe dashboard_data.json de forma atómica.
func WriteDashboard(runDir string, payload OM) error {
	path := filepath.Join(runDir, DashboardFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, OrderedJSON(payload), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
