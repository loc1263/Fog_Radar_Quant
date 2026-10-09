package etf

import (
	"context"
	_ "embed"
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/loc1263/gohoo"
	"github.com/xuri/excelize/v2"

	"fogradar/internal/quant"
	"fogradar/internal/yahoo"
)

// Screener de ETFs (port de Screener_Data_ETF_Script/src/main.py): descarga desde Yahoo precio, AUM, rendimientos por
// período y métricas de riesgo de cada ticker de List_All_ETF.csv y escribe etf_perf_data_<timestamp>.csv (+ xlsx,
// etf_choice_*, List_All_ETF_valid_* e incidencias_*).
//
// Diferencias con el original (solo de eficiencia, no de lógica):
//   - Una sola descarga diaria de 5 años por ticker (en vez de 6 históricos por período); los períodos 5d/1mo/6mo/1y/ytd/5y se
//     derivan con el mismo criterio de Yahoo (primera vela con apertura ≥ ahora − período; 5d = últimas 5 velas). Calibrado
//     contra los rangos nativos de Yahoo (spikes/probe). Son 2 requests por ticker (resumen + chart) en vez de 8.
//   - Sin pausa fija de 30 s cada 30 tickers ni esperas de 60 s/15 min: el cliente de Yahoo común espacia los requests y
//     reintenta los 429 con backoff.

const (
	// DefaultNumETFs: tamaño de etf_choice.
	DefaultNumETFs = 30
	riskFreeRate   = 0.02
	tradingDays    = 252
	marketIndex    = "^GSPC"
	naValue        = "N/A"
	// umbralLento: segundos a partir de los cuales se registra una incidencia LENTO.
	umbralLento = 10 * time.Second
)

//go:embed screener/List_All_ETF.csv
var listAllETF []byte

// ColumnasPerf son las columnas de etf_perf_data (RESULT_FIELDS + TipoInstrumento), en orden.
var ColumnasPerf = []string{"Ticker", "TipoInstrumento", "Nombre", "Precio", "Volumen", "AUM", "Expense Ratio", "Inception Date",
	"Category", "Performance 5D (%)", "Performance 1M (%)", "Performance 6M (%)", "Performance YTD (%)", "Performance 1Y (%)",
	"Performance 5Y (%)", "52W High", "52W Low", "Sharpe Ratio", "Treynor Ratio", "Beta", "Proveedor", "Fecha"}

var providerKeywords = []struct{ kw, name string }{
	{"ishares", "iShares"}, {"blackrock", "iShares"}, {"vanguard", "Vanguard"}, {"spdr", "SPDR"}, {"state street", "SPDR"},
	{"invesco", "Invesco"}, {"global x", "Global X"}, {"vaneck", "VanEck"}, {"schwab", "Charles Schwab"},
	{"jpmorgan", "JPMorgan"}, {"jp morgan", "JPMorgan"},
}

var normalizacionCompany = map[string]string{
	"ishare": "iShares", "ishares": "iShares", "blackrock": "iShares", "vanguard": "Vanguard", "spdr": "SPDR",
	"state street": "SPDR", "invesco": "Invesco", "global x": "Global X", "vaneck": "VanEck", "charles schwab": "Charles Schwab",
	"schwab": "Charles Schwab", "jpmorgan": "JPMorgan", "jp morgan": "JPMorgan", "j.p. morgan": "JPMorgan",
}

// camposCriticosRanking: campos que exige el filtro de completitud del ranking (solo para ETFs).
var camposCriticosRanking = []string{"Performance 5D (%)", "Performance 1M (%)", "Performance 6M (%)", "Performance 1Y (%)",
	"Sharpe Ratio", "Treynor Ratio", "Beta", "52W High", "52W Low", "Volumen", "AUM"}

// --- incidencias ---

// Incidencia es un evento de la corrida del screener (RATE_LIMIT, INFO_INCOMPLETA, ERROR_HISTORIAL, TICKER_DESCARTADO, LENTO).
type Incidencia struct{ Timestamp, Tipo, Ticker, Detalle string }

// Incidencias acumula los eventos de una corrida (thread-safe).
type Incidencias struct {
	mu   sync.Mutex
	list []Incidencia
}

// Registrar agrega un evento.
func (in *Incidencias) Registrar(tipo, ticker, detalle string) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.list = append(in.list, Incidencia{time.Now().Format("2006-01-02 15:04:05"), tipo, ticker, detalle})
}

// Cantidad de incidencias acumuladas.
func (in *Incidencias) Cantidad() int {
	in.mu.Lock()
	defer in.mu.Unlock()
	return len(in.list)
}

// Guardar vuelca las incidencias a <dir>/incidencias_<ts>.csv ("" si no hubo ninguna: no se crea un archivo vacío).
func (in *Incidencias) Guardar(dir, ts string) (string, error) {
	in.mu.Lock()
	list := append([]Incidencia(nil), in.list...)
	in.mu.Unlock()
	if len(list) == 0 {
		return "", nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "incidencias_"+ts+".csv")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(bomUTF8); err != nil {
		return "", err
	}
	w := csv.NewWriter(f)
	w.UseCRLF = true // csv.DictWriter de Python escribe «\r\n»
	_ = w.Write([]string{"Timestamp", "Tipo", "Ticker", "Detalle"})
	for _, i := range list {
		_ = w.Write([]string{i.Timestamp, i.Tipo, i.Ticker, i.Detalle})
	}
	w.Flush()
	return path, w.Error()
}

// describirExcepcion: tipo de error + mensaje truncado a 300 caracteres y sin saltos de línea.
func describirExcepcion(err error) string {
	msg := strings.NewReplacer("\n", " ", "\r", " ").Replace(err.Error())
	if r := []rune(msg); len(r) > 300 {
		msg = string(r[:300]) + "..."
	}
	var se *gohoo.StatusError
	if errors.As(err, &se) {
		return fmt.Sprintf("StatusError (HTTP %d): %s", se.StatusCode, msg)
	}
	return fmt.Sprintf("%T: %s", err, msg)
}

// --- lista de entrada ---

// Meta es la metadata curada de List_All_ETF.csv.
type Meta struct{ Name, Company, Category string }

// LeerLista lee List_All_ETF.csv (separador «;»): tickers únicos y la metadata por ticker.
func LeerLista(data []byte) ([]string, map[string]Meta, error) {
	df, err := quant.ReadCSVSep(strings.NewReader(string(data)), ';')
	if err != nil {
		return nil, nil, err
	}
	tk := df.Col("Ticker")
	if tk == nil {
		return nil, nil, errors.New("el archivo no tiene columna 'Ticker'")
	}
	seen := map[string]bool{}
	var tickers []string
	meta := map[string]Meta{}
	for i := 0; i < df.N; i++ {
		if tk.IsNull(i) {
			continue
		}
		t := tk.Cell(i)
		if !seen[t] {
			seen[t] = true
			tickers = append(tickers, t)
		}
		m := Meta{}
		if c := df.Col("Name"); c != nil && !c.IsNull(i) {
			m.Name = c.Cell(i)
		}
		if c := df.Col("Company"); c != nil && !c.IsNull(i) {
			m.Company = c.Cell(i)
		}
		if c := df.Col("Category"); c != nil && !c.IsNull(i) {
			m.Category = c.Cell(i)
		}
		if _, ok := meta[t]; !ok {
			meta[t] = m
		}
	}
	return tickers, meta, nil
}

// --- períodos y métricas de riesgo ---

// Periodos arma las seis series de la descarga diaria de 5 años con el criterio de Yahoo para cada rango.
type Periodos struct{ P5d, P1m, P6m, P1y, Pytd, P5y []gohoo.Bar }

// DerivarPeriodos recorta las barras (diarias, ordenadas, con la hora de apertura original) a cada rango: 5d = las
// últimas 5 velas; el resto, desde la primera vela con apertura ≥ ahora − período (ytd: ≥ 1 de enero).
func DerivarPeriodos(bars []gohoo.Bar, now time.Time) Periodos {
	from := func(cut time.Time) []gohoo.Bar {
		i := sort.Search(len(bars), func(k int) bool { return !bars[k].Time.Before(cut) })
		return bars[i:]
	}
	n := len(bars)
	loc := time.UTC
	if n > 0 {
		loc = bars[n-1].Time.Location()
	}
	now = now.In(loc)
	p := Periodos{P1m: from(now.AddDate(0, -1, 0)), P6m: from(now.AddDate(0, -6, 0)), P1y: from(now.AddDate(-1, 0, 0)),
		P5y: from(now.AddDate(-5, 0, 0)), Pytd: from(time.Date(now.Year(), 1, 1, 0, 0, 0, 0, loc))}
	p.P5d = bars[max(0, n-5):]
	return p
}

// perfPct: ((precio actual / primer cierre ajustado del período) − 1) · 100; nil si no hay datos.
func perfPct(price float64, bars []gohoo.Bar) *float64 {
	if len(bars) == 0 {
		return nil
	}
	v := (price/bars[0].AdjClose - 1) * 100
	return &v
}

// retornos: pct_change(fill_method=None).dropna() sobre los cierres (descarta los NaN y los que tocan un NaN).
func retornos(bars []gohoo.Bar) (dates []string, r []float64) {
	for i := 1; i < len(bars); i++ {
		v := bars[i].AdjClose/bars[i-1].AdjClose - 1
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			dates = append(dates, bars[i].Time.Format("2006-01-02"))
			r = append(r, v)
		}
	}
	return
}

func round3(x float64) float64 { return round(x, 3) }

// Sharpe: (media·252 − rf) / (desvío·√252); nil si hay < 2 retornos o desvío 0.
func Sharpe(bars []gohoo.Bar) *float64 {
	_, r := retornos(bars)
	if len(r) < 2 {
		return nil
	}
	mean := quant.Mean(r) * tradingDays
	std := quant.StdSample(r) * math.Sqrt(tradingDays)
	if std == 0 || math.IsNaN(std) {
		return nil
	}
	v := round3((mean - riskFreeRate) / std)
	return &v
}

// Beta respecto del mercado con los retornos emparejados POR FECHA (corrección de la Fase 7 documentada en PARIDAD).
func Beta(etf, market []gohoo.Bar) *float64 {
	ed, er := retornos(etf)
	md, mr := retornos(market)
	idx := map[string]float64{}
	for i, d := range md {
		idx[d] = mr[i] // en fechas repetidas gana la última (keep="last")
	}
	var x, y []float64
	seen := map[string]int{}
	for i, d := range ed {
		if m, ok := idx[d]; ok {
			if j, dup := seen[d]; dup {
				x[j], y[j] = er[i], m
				continue
			}
			seen[d] = len(x)
			x, y = append(x, er[i]), append(y, m)
		}
	}
	if len(x) < 2 {
		return nil
	}
	cov := quant.Cov([][]float64{x, y}).At(0, 1)
	variance := quant.Cov([][]float64{y, y}).At(0, 0)
	if variance == 0 {
		return nil
	}
	v := round3(cov / variance)
	return &v
}

// Treynor: (media·252 − rf) / beta; nil si beta es nil o 0.
func Treynor(etf, market []gohoo.Bar) *float64 {
	b := Beta(etf, market)
	if b == nil || *b == 0 {
		return nil
	}
	_, r := retornos(etf)
	if len(r) < 2 {
		return nil
	}
	v := round3((quant.Mean(r)*tradingDays - riskFreeRate) / *b)
	return &v
}

// --- descubrimiento ---

type etfInfo map[string]any

func firstNum(m etfInfo, keys ...string) (float64, bool) {
	for _, k := range keys {
		if f, ok := gohoo.AsFloat(m[k]); ok && f != 0 && !math.IsNaN(f) {
			return f, true
		}
	}
	return 0, false
}

func infoETF(ctx context.Context, src yahoo.Source, ticker string) (etfInfo, error) {
	qs, err := src.QuoteSummary(ctx, ticker, []string{"price", "summaryDetail", "defaultKeyStatistics", "quoteType", "fundProfile"})
	if err != nil {
		return nil, err
	}
	m := etfInfo(yahoo.FlattenInfo(qs))
	if m["quoteType"] == nil {
		if q := gohoo.RawValue(gohoo.GetPath(qs, "price", "quoteType")); q != nil {
			m["quoteType"] = q
		}
	}
	// yfinance entrega netExpenseRatio en puntos porcentuales (0.0945 = 0,0945 %); Yahoo, la fracción en fundProfile.
	if f, ok := gohoo.AsFloat(gohoo.RawValue(gohoo.GetPath(qs, "fundProfile", "feesExpensesInvestment", "annualReportExpenseRatio"))); ok && f != 0 {
		m["netExpenseRatio"] = round(f*100, 4)
	}
	if c, ok := gohoo.GetPath(qs, "fundProfile", "categoryName").(string); ok && c != "" {
		m["category"] = c
	}
	return m, nil
}

// ScreenerResult es el resultado de ProcesarETFs.
type ScreenerResult struct {
	CSV         string
	NValidos    int
	NInvalidos  int
	Incidencias int
}

// ScreenerOpts configura la corrida.
type ScreenerOpts struct {
	OutputDir    string
	ListaCSV     []byte // por defecto la lista embebida
	Concurrencia int
	Log          *slog.Logger
	Now          func() time.Time
}

type validado struct {
	ticker string
	info   etfInfo
	price  float64
	bars   []gohoo.Bar
}

// ProcesarETFs procesa todos los ETFs de la lista y guarda los resultados. Devuelve nil si Yahoo no validó ninguno.
func ProcesarETFs(ctx context.Context, src yahoo.Source, o ScreenerOpts) (*ScreenerResult, error) {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.ListaCSV == nil {
		o.ListaCSV = listAllETF
	}
	if err := os.MkdirAll(o.OutputDir, 0o755); err != nil {
		return nil, err
	}
	tickers, meta, err := LeerLista(o.ListaCSV)
	if err != nil {
		return nil, err
	}
	o.Log.Info("tickers importados", "n", len(tickers))
	inc := &Incidencias{}

	// El historial del índice se pide UNA vez (sin él no se puede calcular Beta/Treynor de ningún ETF).
	o.Log.Info("precargando historial de mercado", "indice", marketIndex)
	mkt, err := src.Chart(ctx, marketIndex, gohoo.ChartParams{Range: "1y", Interval: "1d"})
	if err != nil {
		return nil, fmt.Errorf("no se pudo precargar el historial de %s: %w", marketIndex, err)
	}
	market := yahoo.AutoAdjust(mkt.Bars)

	type out struct {
		row []any
		ok  bool
	}
	results := make([]out, len(tickers))
	var done int
	var dmu sync.Mutex
	conc := max(1, o.Concurrencia)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < conc; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if ctx.Err() != nil {
					continue
				}
				row := procesarTicker(ctx, src, tickers[i], meta[tickers[i]], market, inc, o)
				results[i] = out{row, row != nil}
				dmu.Lock()
				done++
				pct := float64(done) / float64(len(tickers)) * 100
				if row != nil {
					o.Log.Info("ETF", "n", fmt.Sprintf("%d/%d | %5.1f%%", done, len(tickers), pct), "ticker", tickers[i])
				} else {
					o.Log.Info("ETF inválido", "n", fmt.Sprintf("%d/%d | %5.1f%%", done, len(tickers), pct), "ticker", tickers[i])
				}
				dmu.Unlock()
			}
		}()
	}
	for i := range tickers {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	var rows [][]any
	invalid := 0
	for _, r := range results {
		if r.ok {
			rows = append(rows, r.row)
		} else {
			invalid++
		}
	}
	o.Log.Info("validación", "validos", len(rows), "invalidos", invalid)
	if len(rows) == 0 {
		o.Log.Warn("No se validó ningún ETF")
		return nil, nil
	}
	df := quant.NewFrame(len(rows))
	for j, name := range ColumnasPerf[:len(ColumnasPerf)-1] { // sin «Fecha»: se agrega al guardar
		vals := make([]any, len(rows))
		for i := range rows {
			vals[i] = rows[i][j]
		}
		df.Set(quant.Cells(name, vals))
	}
	ts := o.Now().Format("20060102_150405")
	res, err := guardarPerfData(df, ts, o.OutputDir, o.Now)
	if err != nil {
		return nil, err
	}
	if _, err := guardarValidos(df, ts, o.OutputDir, o.ListaCSV, o.Now); err != nil {
		o.Log.Warn("no se pudo guardar la lista de tickers válidos", "error", err)
	}
	if p, err := inc.Guardar(o.OutputDir, ts); err != nil {
		o.Log.Warn("no se pudieron guardar las incidencias", "error", err)
	} else if p != "" {
		o.Log.Info("incidencias", "n", inc.Cantidad(), "archivo", filepath.Base(p))
	}
	return &ScreenerResult{CSV: res, NValidos: len(rows), NInvalidos: invalid, Incidencias: inc.Cantidad()}, nil
}

// procesarTicker valida un ETF (reintentos) y arma su fila; nil si se descarta.
func procesarTicker(ctx context.Context, src yahoo.Source, ticker string, m Meta, market []gohoo.Bar, inc *Incidencias, o ScreenerOpts) []any {
	start := time.Now()
	defer func() {
		if d := time.Since(start); d > umbralLento {
			inc.Registrar("LENTO", ticker, fmt.Sprintf("proceso completo tardó %.1fs (umbral: %.0fs)", d.Seconds(), umbralLento.Seconds()))
		}
	}()
	const maxRetries = 3
	var v *validado
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		v, lastErr = validar(ctx, src, ticker)
		if lastErr == nil {
			break
		}
		if ctx.Err() != nil || errors.Is(lastErr, gohoo.ErrNotFound) {
			break
		}
		if gohoo.IsRateLimit(lastErr) {
			inc.Registrar("RATE_LIMIT", ticker, fmt.Sprintf("intento %d/%d. %s", attempt+1, maxRetries, describirExcepcion(lastErr)))
		}
		select {
		case <-time.After(time.Duration(attempt+1) * time.Second):
		case <-ctx.Done():
		}
	}
	if lastErr != nil {
		if !errors.Is(lastErr, gohoo.ErrNotFound) {
			inc.Registrar("TICKER_DESCARTADO", ticker, fmt.Sprintf("se agotaron los %d intentos de validacion. Ultimo error: %s", maxRetries, describirExcepcion(lastErr)))
		}
		return nil
	}
	if v == nil {
		return nil // sin datos útiles (precio o historial vacío)
	}
	return construirFila(v, m, market, inc, o.Now())
}

// validar obtiene info + historial; (nil, nil) si el instrumento no tiene precio o historial.
func validar(ctx context.Context, src yahoo.Source, ticker string) (*validado, error) {
	info, err := infoETF(ctx, src, ticker)
	if err != nil {
		return nil, err
	}
	price, ok := firstNum(info, "regularMarketPrice", "currentPrice", "previousClose", "regularMarketPreviousClose")
	if len(info) == 0 || !ok {
		return nil, nil
	}
	res, err := src.Chart(ctx, ticker, gohoo.ChartParams{Range: "5y", Interval: "1d"})
	if err != nil {
		return nil, err
	}
	if len(res.Bars) == 0 {
		return nil, nil
	}
	// AdjClose queda como cierre ajustado (auto_adjust); Time conserva la hora de apertura para derivar los rangos
	bars := make([]gohoo.Bar, len(res.Bars))
	copy(bars, res.Bars)
	return &validado{ticker: ticker, info: info, price: price, bars: bars}, nil
}

func infoVal(m etfInfo, k string) any {
	v, ok := m[k]
	if !ok || v == nil {
		return naValue
	}
	return v
}

// intIfWhole: los enteros de Yahoo (volumen, AUM, fecha de inicio) se escriben sin decimales.
func intIfWhole(v any) any {
	if f, ok := v.(float64); ok && f == math.Trunc(f) && math.Abs(f) < 1e18 {
		return int64(f)
	}
	return v
}

func fmtPerf(p *float64) any {
	if p == nil || math.IsNaN(*p) {
		return naValue
	}
	return round(*p, 2)
}

func optVal(p *float64) any {
	if p == nil {
		return naValue
	}
	return *p
}

func getProvider(nombre string, company string) string {
	if company != "" {
		if n, ok := normalizacionCompany[strings.ToLower(strings.TrimSpace(company))]; ok {
			return n
		}
		return company
	}
	lower := strings.ToLower(nombre)
	for _, p := range providerKeywords {
		if strings.Contains(lower, p.kw) {
			return p.name
		}
	}
	return "Otro"
}

// construirFila arma la fila de resultado (en el orden de ColumnasPerf sin «Fecha»).
func construirFila(v *validado, m Meta, market []gohoo.Bar, inc *Incidencias, now time.Time) []any {
	info := v.info
	per := DerivarPeriodos(v.bars, now)
	p1y := yahoo.AutoAdjust(per.P1y)
	// los rendimientos usan el cierre ajustado de la primera vela del período
	perf := func(b []gohoo.Bar) any { return fmtPerf(perfPct(v.price, b)) }

	tipo := ""
	if s, ok := info["quoteType"].(string); ok {
		tipo = s
	}
	expense := any(naValue)
	if f, ok := firstNum(info, "netExpenseRatio", "annualReportExpenseRatio", "totalExpenseRatio", "expenseRatio"); ok {
		expense = f
	}
	nombre := ""
	if s, ok := info["longName"].(string); ok && s != "" {
		nombre = s
	} else if s, ok := info["shortName"].(string); ok && s != "" {
		nombre = s
	}
	if nombre == "" && m.Name != "" {
		nombre = m.Name
	}
	var nombreVal any = naValue
	if nombre != "" {
		nombreVal = nombre
	}
	cat := any(naValue)
	if m.Category != "" {
		cat = m.Category
	} else if c, ok := info["category"].(string); ok && c != "" {
		cat = c
	}
	provName := nombre
	if provName == "" {
		provName = naValue
	}
	sharpe, treynor, beta := Sharpe(p1y), Treynor(p1y, market), Beta(p1y, market)

	row := map[string]any{
		"Ticker": v.ticker, "TipoInstrumento": tipo, "Nombre": nombreVal, "Precio": round(v.price, 2),
		"Volumen": intIfWhole(infoVal(info, "volume")), "AUM": intIfWhole(infoVal(info, "totalAssets")), "Expense Ratio": expense,
		"Inception Date": intIfWhole(infoVal(info, "fundInceptionDate")), "Category": cat,
		"Performance 5D (%)": perf(per.P5d), "Performance 1M (%)": perf(yahoo.AutoAdjust(per.P1m)),
		"Performance 6M (%)": perf(yahoo.AutoAdjust(per.P6m)), "Performance YTD (%)": perf(yahoo.AutoAdjust(per.Pytd)),
		"Performance 1Y (%)": perf(p1y), "Performance 5Y (%)": perf(yahoo.AutoAdjust(per.P5y)),
		"52W High": infoVal(info, "fiftyTwoWeekHigh"), "52W Low": infoVal(info, "fiftyTwoWeekLow"),
		"Sharpe Ratio": optVal(sharpe), "Treynor Ratio": optVal(treynor), "Beta": optVal(beta),
		"Proveedor": getProvider(provName, m.Company),
	}
	campos := []string{"Nombre"}
	if tipo == "ETF" {
		campos = append(campos, camposCriticosRanking...)
	}
	var faltan []string
	for _, c := range campos {
		if x := row[c]; x == naValue || x == nil {
			faltan = append(faltan, c)
		}
	}
	if len(faltan) > 0 {
		inc.Registrar("INFO_INCOMPLETA", v.ticker, "Yahoo no devolvio: "+strings.Join(faltan, ", ")+". Salvo Nombre, estos son campos que exige "+
			"el filtro de completitud del ranking -- es probable que este ticker quede afuera del universo depurado en esta corrida.")
	}
	out := make([]any, len(ColumnasPerf)-1)
	for i, c := range ColumnasPerf[:len(ColumnasPerf)-1] {
		out[i] = row[c]
	}
	return out
}

// --- escritura ---

func writeCSVBOM(path string, df *quant.Frame, sep byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(bomUTF8); err != nil {
		f.Close()
		return err
	}
	if err := df.WriteCSVSep(f, sep); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func writeXLSXFrame(path string, df *quant.Frame) error {
	x := excelize.NewFile()
	defer func() { _ = x.Close() }()
	for j, c := range df.Cols {
		cell, _ := excelize.CoordinatesToCellName(j+1, 1)
		_ = x.SetCellValue("Sheet1", cell, c.Name)
		for i := 0; i < df.N; i++ {
			if v := c.Value(i); v != nil {
				cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
				_ = x.SetCellValue("Sheet1", cell, v)
			}
		}
	}
	return x.SaveAs(path)
}

func withFecha(df *quant.Frame, now time.Time) *quant.Frame {
	out := df.Copy()
	fechas := make([]string, out.N)
	for i := range fechas {
		fechas[i] = now.Format("2006-01-02")
	}
	out.SetStrs("Fecha", fechas)
	return out
}

// guardarPerfData escribe etf_perf_data_<ts>.csv (+ xlsx) y etf_choice_<ts>.csv (+ xlsx); devuelve la ruta del CSV.
func guardarPerfData(df *quant.Frame, ts, dir string, now func() time.Time) (string, error) {
	full := withFecha(df, now())
	csvPath := filepath.Join(dir, "etf_perf_data_"+ts+".csv")
	if err := writeCSVBOM(csvPath, full, ','); err != nil {
		return "", err
	}
	_ = writeXLSXFrame(filepath.Join(dir, "etf_perf_data_"+ts+".xlsx"), full) // el xlsx es opcional (como en el original)
	return csvPath, guardarEtfChoice(csvPath, ts, dir, now)
}

// guardarEtfChoice: los mejores ETFs con rendimiento creciente y no negativo en 5D→5Y, ordenados por 5Y (top 30).
func guardarEtfChoice(csvPath, ts, dir string, now func() time.Time) error {
	df, err := quant.ReadCSVFile(csvPath)
	if err != nil || !df.Has("TipoInstrumento") {
		return err
	}
	etf := df.Filter(tipoETF(df))
	var periodos []string
	for _, c := range []string{"Performance 5D (%)", "Performance 1M (%)", "Performance 6M (%)", "Performance 1Y (%)", "Performance 5Y (%)"} {
		if etf.Has(c) {
			periodos = append(periodos, c)
		}
	}
	if len(periodos) < 2 {
		return nil
	}
	keep := make([]bool, etf.N)
	cols := make([][]float64, len(periodos))
	for k, p := range periodos {
		cols[k] = etf.Col(p).Floats()
	}
	for i := range keep {
		ok := true
		for k := range periodos {
			v := cols[k][i]
			if math.IsNaN(v) || v < 0 || (k > 0 && cols[k-1][i] > v) {
				ok = false
			}
		}
		keep[i] = ok
	}
	sel := etf.Filter(keep)
	last := sel.Col(periodos[len(periodos)-1]).Floats()
	top := sel.Take(quant.OrderDesc(last)).Head(DefaultNumETFs)
	return writeCSVBOM(filepath.Join(dir, "etf_choice_"+ts+".csv"), withFecha(top, now()), ';')
}

// guardarValidos escribe List_All_ETF_valid_<ts>.csv si algún ticker de la lista no validó ("" si todos son válidos).
func guardarValidos(df *quant.Frame, ts, dir string, lista []byte, now func() time.Time) (string, error) {
	orig, err := quant.ReadCSVSep(strings.NewReader(string(lista)), ';')
	if err != nil {
		return "", err
	}
	valid := map[string]bool{}
	for i := 0; i < df.N; i++ {
		valid[df.Col("Ticker").Cell(i)] = true
	}
	tk := orig.Col("Ticker")
	keep := make([]bool, orig.N)
	n := 0
	for i := range keep {
		keep[i] = !tk.IsNull(i) && valid[tk.Cell(i)]
		if keep[i] {
			n++
		}
	}
	if n >= orig.N {
		return "", nil
	}
	v := orig.Filter(keep).Copy()
	last := v.Cols[len(v.Cols)-1]
	hoy := make([]string, v.N)
	for i := range hoy {
		hoy[i] = now().Format("20060102")
	}
	v.SetStrs(last.Name, hoy)
	path := filepath.Join(dir, "List_All_ETF_valid_"+ts+".csv")
	return path, writeCSVBOM(path, v, ';')
}

// bomUTF8 es la marca de orden de bytes que escribe encoding="utf-8-sig".
var bomUTF8 = string(rune(0xFEFF))
