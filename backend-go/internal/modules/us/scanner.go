package us

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/loc1263/gohoo"

	"fogradar/internal/core"
	"fogradar/internal/jobs"
	"fogradar/internal/yahoo"
)

// Escaneo cuantitativo, pensado para correr como batch (fuera de cualquier request HTTP). Son 7 pasos
// secuenciales y cada uno hace UNA sola cosa: o descarga datos (red a Yahoo) o evalúa sobre datos ya
// descargados (función pura):
//
//  1. universo de tickers (S&P 500 ∪ Nasdaq-100)        — descarga
//  2. market cap de cada ticker del universo            — descarga (quoteSummary «price»)
//  3. filtro por umbral de market cap                   — puro
//  4. histórico de precios de los candidatos            — descarga (chart 14 meses, concurrente)
//  5. momentum 12-1, performance por plazo e indicadores — puro
//  6. fundamentales de cada candidato                   — descarga (info + trailingPegRatio)
//  7. fundamentales + score de salud                    — puro
//
// Cada corrida deja result.json, candidates.json, prices.json y run.log en data/us_stocks/runs/<run_id>/.
const (
	DefaultMinMarketCap = 200_000_000_000 // 200B USD
	DefaultPauseSeconds = 0.6             // se registra por compatibilidad; el espaciado lo hace el cliente de Yahoo
	TopN                = 15
	MaxRetries          = 2
	HistoryRange        = "14mo"
)

// Momentum es el resultado del paso 5 para un ticker.
type Momentum struct {
	Price       float64
	Momentum121 float64
	Return1d    *float64
	Return5d    *float64
	Return3m    *float64
	Return6m    *float64
	Return1y    *float64
	ReturnYtd   *float64
}

// priceAt es el precio de cierre vigente en target (el último dato en o antes de esa fecha); nil si
// target cae antes del histórico o el dato es inválido.
func priceAt(times []time.Time, closes []float64, target time.Time) *float64 {
	if target.Before(times[0]) {
		return nil
	}
	i := sort.Search(len(times), func(i int) bool { return times[i].After(target) }) - 1
	if i < 0 {
		return nil
	}
	v := closes[i]
	if math.IsNaN(v) || v <= 0 {
		return nil
	}
	return &v
}

// ComputeMomentum: momentum 12-1 y performance en varios plazos desde el histórico ya descargado.
// nil si no hay al menos ~12 meses de histórico (sin eso el momentum 12-1 no es fiable). Cada plazo de
// performance se calcula por separado y puede quedar en nil sin invalidar el resto.
func ComputeMomentum(times []time.Time, closesIn []float64) *Momentum {
	var ts []time.Time
	var closes []float64
	for i, c := range closesIn {
		if !math.IsNaN(c) {
			ts = append(ts, times[i])
			closes = append(closes, c)
		}
	}
	if len(closes) == 0 {
		return nil
	}
	last, first := ts[len(ts)-1], ts[0]
	if int(last.Sub(first).Hours()/24) < 300 {
		return nil
	}
	pNow := closes[len(closes)-1]
	if math.IsNaN(pNow) || pNow <= 0 {
		return nil
	}
	day := 24 * time.Hour
	p1mo := priceAt(ts, closes, last.Add(-30*day))
	p12mo := priceAt(ts, closes, last.Add(-365*day))
	if p1mo == nil || p12mo == nil {
		return nil
	}
	ret := func(p *float64) *float64 {
		if p == nil || *p == 0 {
			return nil
		}
		v := pNow/(*p) - 1
		return &v
	}
	y := time.Date(last.Year(), 1, 1, 0, 0, 0, 0, last.Location())
	return &Momentum{
		Price: pNow, Momentum121: (*p1mo / *p12mo) - 1,
		Return1d: ret(priceAt(ts, closes, last.Add(-1*day))), Return5d: ret(priceAt(ts, closes, last.Add(-5*day))),
		Return3m: ret(priceAt(ts, closes, last.Add(-91*day))), Return6m: ret(priceAt(ts, closes, last.Add(-182*day))),
		Return1y: ret(p12mo), ReturnYtd: ret(priceAt(ts, closes, y)),
	}
}

// BuildPriceMatrix serializa los cierres de los candidatos evaluados a una matriz fechas × tickers (para que
// el portafolio calcule covarianza/correlación sin volver a pedirle nada a Yahoo). dates es la unión de las
// fechas de todas las series descargadas, como el índice de yf.download.
func BuildPriceMatrix(evaluated []string, bars map[string][]gohoo.Bar, allTickers []string) PriceMatrix {
	set := map[string]bool{}
	for _, t := range allTickers {
		for _, b := range bars[t] {
			set[b.Time.Format("2006-01-02")] = true
		}
	}
	dates := make([]string, 0, len(set))
	for d := range set {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	pos := make(map[string]int, len(dates))
	for i, d := range dates {
		pos[d] = i
	}
	prices := map[string][]*float64{}
	for _, t := range evaluated {
		b, ok := bars[t]
		if !ok {
			continue
		}
		col := make([]*float64, len(dates))
		for _, bar := range b {
			if !math.IsNaN(bar.Close) {
				v := bar.Close
				col[pos[bar.Time.Format("2006-01-02")]] = &v
			}
		}
		prices[t] = col
	}
	return PriceMatrix{Dates: dates, Prices: prices}
}

// --- orquestación ---

type runLog struct {
	console *slog.Logger
	file    *slog.Logger
}

func (l *runLog) info(msg string, args ...any) {
	l.console.Info(msg, args...)
	l.file.Info(msg, args...)
}

func (l *runLog) warn(msg string, args ...any) {
	l.console.Warn(msg, args...)
	l.file.Warn(msg, args...)
}

// workers corre fn(i) para i en [0,n) con concurrencia acotada.
func workers(ctx context.Context, n, conc int, fn func(i int)) {
	conc = max(1, min(conc, n))
	jobsCh := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < conc; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobsCh {
				if ctx.Err() == nil {
					fn(i)
				}
			}
		}()
	}
	for i := 0; i < n; i++ {
		jobsCh <- i
	}
	close(jobsCh)
	wg.Wait()
}

func retrying[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var v T
	var err error
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		v, err = fn()
		if err == nil || errors.Is(err, gohoo.ErrNotFound) || ctx.Err() != nil {
			return v, err
		}
		if attempt < MaxRetries {
			select {
			case <-time.After(time.Duration(800*(attempt+1)) * time.Millisecond):
			case <-ctx.Done():
				return v, ctx.Err()
			}
		}
	}
	return v, err
}

func writeJSON(path string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// BatchOpts configura RunBatch.
type BatchOpts struct {
	MinMarketCap float64
	OutputRoot   string
}

// RunBatch ejecuta una corrida completa y deja todos sus archivos en una carpeta propia bajo OutputRoot.
// Si un paso falla, se corta y se persiste lo evaluado hasta ese momento con status «error».
func RunBatch(ctx context.Context, env *jobs.Env, o BatchOpts) (string, error) {
	src := env.Yahoo
	if o.MinMarketCap == 0 {
		o.MinMarketCap = DefaultMinMarketCap
	}
	started := time.Now().UTC()
	runID := started.Format("2006-01-02T15-04-05Z")
	runDir := filepath.Join(o.OutputRoot, runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", err
	}
	lf, err := os.Create(filepath.Join(runDir, "run.log"))
	if err != nil {
		return "", err
	}
	defer lf.Close()
	lg := &runLog{console: env.Log, file: slog.New(slog.NewTextHandler(lf, nil))}
	conc := env.S.YahooConcurrencia

	status, errMsg := "done", (*string)(nil)
	var universe, candidates []string
	var topRows, evaluated []CandidateRow
	var matrix *PriceMatrix

	runErr := func() error {
		lg.info("Paso 1/7: obteniendo universo de acciones de EEUU...")
		universe = LargeCapUniverse(ctx, func(f string, a ...any) { lg.warn(fmt.Sprintf(f, a...)) })
		for i, t := range universe {
			universe[i] = CleanTicker(t)
		}
		lg.info("Universo", "tickers", len(universe))

		lg.info("Paso 2/7: descargando market cap", "tickers", len(universe))
		caps := make([]*float64, len(universe))
		var done int
		var dmu sync.Mutex
		workers(ctx, len(universe), conc, func(i int) {
			v, err := retrying(ctx, func() (*float64, error) {
				qs, err := src.QuoteSummary(ctx, universe[i], []string{"price"})
				if err != nil {
					return nil, err
				}
				if f, ok := gohoo.AsFloat(gohoo.RawValue(gohoo.GetPath(qs, "price", "marketCap"))); ok && f > 0 {
					return &f, nil
				}
				return nil, nil
			})
			if err != nil {
				lg.warn("No se pudo obtener market cap", "ticker", universe[i], "error", err)
			}
			caps[i] = v
			dmu.Lock()
			done++
			if done%25 == 0 || done == len(universe) {
				lg.info("market caps descargados", "n", fmt.Sprintf("%d/%d", done, len(universe)))
			}
			dmu.Unlock()
		})
		if ctx.Err() != nil {
			return ctx.Err()
		}

		lg.info("Paso 3/7: evaluando market cap contra el umbral", "tickers", len(universe))
		capOf := map[string]*float64{}
		for i, t := range universe {
			capOf[t] = caps[i]
			if caps[i] != nil && *caps[i] >= o.MinMarketCap {
				candidates = append(candidates, t)
			}
		}
		lg.info("tickers que pasan el filtro de market cap", "n", fmt.Sprintf("%d/%d", len(candidates), len(universe)))
		if len(candidates) == 0 {
			lg.info("Ningún ticker superó el umbral de market cap.")
			return nil
		}

		lg.info("Paso 4/7: descargando histórico de precios", "candidatos", len(candidates))
		hist := make([][]gohoo.Bar, len(candidates))
		workers(ctx, len(candidates), conc, func(i int) {
			res, err := retrying(ctx, func() (*gohoo.ChartResult, error) {
				return src.Chart(ctx, candidates[i], gohoo.ChartParams{Range: HistoryRange, Interval: "1d"})
			})
			if err != nil {
				lg.warn("sin histórico", "ticker", candidates[i], "error", err)
				return
			}
			hist[i] = yahoo.AutoAdjust(res.Bars)
		})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		bars := map[string][]gohoo.Bar{}
		for i, t := range candidates {
			if len(hist[i]) > 0 {
				bars[t] = hist[i]
			}
		}

		lg.info("Paso 5/7: calculando momentum, performance e indicadores técnicos", "candidatos", len(candidates))
		var rows []CandidateRow
		for _, t := range candidates {
			b, ok := bars[t]
			if !ok {
				continue
			}
			times := make([]time.Time, len(b))
			closes, vols := make([]float64, len(b)), make([]float64, len(b))
			for i, x := range b {
				times[i], closes[i], vols[i] = x.Time, x.Close, x.Volume
			}
			m := ComputeMomentum(times, closes)
			if m == nil {
				continue
			}
			var clean []float64
			for _, c := range closes {
				if !math.IsNaN(c) {
					clean = append(clean, c)
				}
			}
			rows = append(rows, CandidateRow{Ticker: t, MarketCap: capOf[t], Price: m.Price, Momentum121: m.Momentum121,
				Return1d: m.Return1d, Return5d: m.Return5d, Return3m: m.Return3m, Return6m: m.Return6m,
				Return1y: m.Return1y, ReturnYtd: m.ReturnYtd, Technical: BuildTechnical(clean, vols)})
		}
		lg.info("candidatos con histórico suficiente para calcular momentum", "n", fmt.Sprintf("%d/%d", len(rows), len(candidates)))
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Momentum121 > rows[j].Momentum121 })

		lg.info("Paso 6/7: descargando fundamentales", "candidatos", len(rows))
		infos := make([]map[string]any, len(rows))
		workers(ctx, len(rows), conc, func(i int) {
			t := rows[i].Ticker
			info, err := retrying(ctx, func() (map[string]any, error) { return yahoo.Info(ctx, src, t) })
			if err != nil {
				lg.warn("fundamentales no disponibles", "ticker", t, "error", err)
				return
			}
			if v, ok, perr := yahoo.TrailingPEG(ctx, src, t); perr == nil && ok {
				info["trailingPegRatio"] = v
			}
			infos[i] = info
			lg.info("fundamentales OK", "n", fmt.Sprintf("%d/%d", i+1, len(rows)), "ticker", t)
		})
		if ctx.Err() != nil {
			return ctx.Err()
		}

		lg.info("Paso 7/7: evaluando fundamentales y calculando score", "candidatos", len(rows))
		for i := range rows {
			r := &rows[i]
			name, sector := r.Ticker, ""
			if info := infos[i]; len(info) > 0 {
				if n := strOr(info, "longName", "shortName"); n != "" {
					name = n
				}
				sector = strOr(info, "sector")
				r.Valuation, r.Profitability = BuildValuation(info), BuildProfitability(info)
				r.FinancialHealth, r.ShareholderReturn = BuildFinancialHealth(info), BuildShareholderReturn(info)
			}
			r.Name, r.Sector = &name, &sector
			p := r.Price
			r.Score = EvaluateScore(Evaluation{Price: &p, MarketCap: r.MarketCap, Valuation: r.Valuation, Profitability: r.Profitability,
				FinancialHealth: r.FinancialHealth, ShareholderReturn: r.ShareholderReturn, Technical: r.Technical})
			lg.info("evaluado", "ticker", r.Ticker, "score", derefF(r.Score.Overall))
		}
		evaluated = rows
		topRows = append([]CandidateRow(nil), rows[:min(TopN, len(rows))]...)
		names := make([]string, len(rows))
		for i, r := range rows {
			names[i] = r.Ticker
		}
		m := BuildPriceMatrix(names, bars, candidates)
		matrix = &m
		lg.info("Corrida completa", "acciones_en_ranking", len(topRows))
		return nil
	}()
	if runErr != nil {
		status = "error"
		m := runErr.Error()
		errMsg = &m
		env.Log.Error("la corrida falló", "run_id", runID, "error", runErr)
		lg.file.Error("la corrida falló", "error", runErr)
	}

	meta := RunMetadata{RunID: runID, Status: status, Error: errMsg, StartedAt: core.Fecha{Time: started},
		FinishedAt: core.Fecha{Time: time.Now().UTC()}, MinMarketCap: o.MinMarketCap, PauseSeconds: DefaultPauseSeconds,
		UniverseTotal: int64(len(universe)), CandidatesTotal: int64(len(candidates)), TopN: TopN}
	if topRows == nil {
		topRows = []CandidateRow{}
	}
	if evaluated == nil {
		evaluated = []CandidateRow{}
	}
	if err := writeJSON(filepath.Join(runDir, "result.json"), RunResponse{RunMetadata: meta, Rows: topRows}); err != nil {
		return runDir, err
	}
	if err := writeJSON(filepath.Join(runDir, "candidates.json"), RunResponse{RunMetadata: meta, Rows: evaluated}); err != nil {
		return runDir, err
	}
	if matrix != nil {
		if err := writeJSON(filepath.Join(runDir, "prices.json"), matrix); err != nil {
			return runDir, err
		}
	}
	return runDir, nil
}

// MarshalIndentJSON (tests) serializa como los archivos de corrida.
func MarshalIndentJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(v)
	return bytes.TrimRight(buf.Bytes(), "\n"), err
}

func derefF(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}
