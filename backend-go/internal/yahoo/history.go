package yahoo

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/loc1263/gohoo"
)

// HistoryOptions son las opciones de descarga de históricos.
type HistoryOptions struct {
	Range       string
	Interval    string
	MaxRetries  int
	PauseMin    time.Duration // pausa tras cada descarga exitosa (en Go el espaciado global ya protege: 0 por defecto)
	PauseMax    time.Duration
	BackoffBase time.Duration
	BackoffMax  time.Duration
	Timeout     time.Duration
	CacheMaxAge time.Duration
}

// DefaultHistoryOptions son los valores por defecto (5y semanal, 5 intentos, caché de 20 h).
func DefaultHistoryOptions() HistoryOptions {
	return HistoryOptions{Range: "5y", Interval: "1wk", MaxRetries: 5,
		BackoffBase: 5 * time.Second, BackoffMax: 120 * time.Second, Timeout: 30 * time.Second,
		CacheMaxAge: 20 * time.Hour}
}

// CachePath es la ruta del CSV en caché de un ticker (^ y = no valen en nombres de archivo).
func CachePath(dir, ticker string) string {
	safe := strings.NewReplacer("^", "_", "=", "_").Replace(ticker)
	return filepath.Join(dir, safe+".csv")
}

// CacheFresh dice si el archivo existe y es más nuevo que maxAge.
func CacheFresh(path string, maxAge time.Duration) bool {
	st, err := os.Stat(path)
	return err == nil && time.Since(st.ModTime()) < maxAge
}

// History devuelve el histórico de un ticker
// con caché en disco, reintentos y escritura atómica. El CSV usa el formato
// (Date,Open,High,Low,Close,Volume,Dividends,Stock Splits). Devuelve error en vez de un
// DataFrame vacío, y no reintenta los símbolos inexistentes (ErrNotFound).
func (c *Client) History(ctx context.Context, ticker, cacheDir string, o HistoryOptions, force bool) ([]gohoo.Bar, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	file := CachePath(cacheDir, ticker)
	if !force && CacheFresh(file, o.CacheMaxAge) {
		if bars, err := ReadBarsCSV(file); err == nil && len(bars) > 0 {
			return bars, nil
		}
		// caché corrupta: se vuelve a descargar
	}

	var last error
	for attempt := 0; attempt < max(1, o.MaxRetries); attempt++ {
		cctx, cancel := ctx, context.CancelFunc(func() {})
		if o.Timeout > 0 {
			cctx, cancel = context.WithTimeout(ctx, o.Timeout)
		}
		res, err := c.Chart(cctx, ticker, gohoo.ChartParams{Range: o.Range, Interval: o.Interval})
		cancel()
		if err == nil && (res == nil || len(res.Bars) == 0) {
			err = errors.New("respuesta vacía de Yahoo Finance")
		}
		if err == nil {
			bars := AutoAdjust(res.Bars)
			if werr := WriteBarsCSV(file, bars, res.Dividends, res.Splits); werr != nil {
				return nil, werr
			}
			if o.PauseMax > 0 {
				p := o.PauseMin + time.Duration(rand.Float64()*float64(o.PauseMax-o.PauseMin))
				if serr := sleepCtx(ctx, p); serr != nil {
					return bars, nil // cancelado durante la pausa: el dato ya está guardado
				}
			}
			return bars, nil
		}
		last = err
		if ctx.Err() != nil || errors.Is(err, gohoo.ErrNotFound) {
			break
		}
		if attempt < o.MaxRetries-1 {
			if serr := sleepCtx(ctx, Backoff(attempt, o.BackoffBase, o.BackoffMax, nil)); serr != nil {
				break
			}
		}
	}
	return nil, fmt.Errorf("%s: %w", ticker, last)
}

// Backoff es min(base·2^intento, max) + U(0, 1) s (reintento externo de History).
func Backoff(attempt int, base, maxD time.Duration, rnd func() float64) time.Duration {
	if rnd == nil {
		rnd = rand.Float64
	}
	d := base << min(attempt, 30)
	if d <= 0 || d > maxD {
		d = maxD
	}
	return d + time.Duration(rnd()*float64(time.Second))
}

// AutoAdjust deja Close = AdjClose, escala Open/High/Low con el mismo factor y lleva la hora de cada
// barra a la medianoche local (las velas diarias y semanales por fecha, no por hora de apertura): las
// comparaciones por fecha («último dato en o antes de…») dependen de eso.
func AutoAdjust(in []gohoo.Bar) []gohoo.Bar { return gohoo.AtMidnight(gohoo.AutoAdjust(in)) }

const csvTimeLayout = "2006-01-02 15:04:05-07:00"

func fmtFloat(v float64) string {
	if math.IsNaN(v) {
		return ""
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// WriteBarsCSV escribe el CSV de forma atómica (archivo temporal + rename).
func WriteBarsCSV(path string, bars []gohoo.Bar, dividends, splits map[string]float64) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	_, _ = fmt.Fprintln(w, "Date,Open,High,Low,Close,Volume,Dividends,Stock Splits")
	for _, b := range bars {
		day := b.Time.Format("2006-01-02")
		_, _ = fmt.Fprintf(w, "%s,%s,%s,%s,%s,%s,%s,%s\n", b.Time.Format(csvTimeLayout),
			fmtFloat(b.Open), fmtFloat(b.High), fmtFloat(b.Low), fmtFloat(b.Close), fmtFloat(b.Volume),
			strconv.FormatFloat(dividends[day], 'g', -1, 64), strconv.FormatFloat(splits[day], 'g', -1, 64))
	}
	if err := w.Flush(); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

var csvTimeLayouts = []string{csvTimeLayout, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05", "2006-01-02"}

func parseCSVTime(s string) (time.Time, error) {
	for _, l := range csvTimeLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("fecha inválida %q", s)
}

// ReadBarsCSV lee un CSV de barras (la primera columna es la fecha; busca las columnas por nombre).
func ReadBarsCSV(path string) ([]gohoo.Bar, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	if !sc.Scan() {
		return nil, errors.New("CSV vacío")
	}
	idx := map[string]int{}
	for i, h := range strings.Split(strings.TrimSpace(sc.Text()), ",") {
		idx[strings.Trim(h, `"`)] = i
	}
	if _, ok := idx["Close"]; !ok {
		return nil, errors.New("el CSV no tiene la columna Close")
	}
	var bars []gohoo.Bar
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		cols := strings.Split(line, ",")
		t, err := parseCSVTime(strings.Trim(cols[0], `"`))
		if err != nil {
			return nil, err
		}
		get := func(name string) float64 {
			i, ok := idx[name]
			if !ok || i >= len(cols) || cols[i] == "" {
				return math.NaN()
			}
			v, err := strconv.ParseFloat(cols[i], 64)
			if err != nil {
				return math.NaN()
			}
			return v
		}
		c := get("Close")
		bars = append(bars, gohoo.Bar{Time: t, Open: get("Open"), High: get("High"), Low: get("Low"),
			Close: c, AdjClose: c, Volume: get("Volume")})
	}
	return bars, sc.Err()
}

// Progress se llama antes de descargar cada ticker (i desde 1).
type Progress func(i, n int, ticker string)

// DownloadMany descarga varios tickers con concurrencia acotada (los inicios de request siguen
// espaciados por el Spacer global). Devuelve las series y los tickers fallidos en el orden de entrada.
func (c *Client) DownloadMany(ctx context.Context, tickers []string, cacheDir string, o HistoryOptions,
	force bool, workers int, progress Progress) (map[string][]gohoo.Bar, []string) {
	workers = max(1, min(workers, len(tickers)))
	type res struct {
		bars []gohoo.Bar
		err  error
	}
	results := make([]res, len(tickers))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if progress != nil {
					progress(i+1, len(tickers), tickers[i])
				}
				b, err := c.History(ctx, tickers[i], cacheDir, o, force)
				results[i] = res{b, err}
			}
		}()
	}
	for i := range tickers {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	out := make(map[string][]gohoo.Bar, len(tickers))
	var failed []string
	for i, t := range tickers {
		if results[i].err != nil || len(results[i].bars) == 0 {
			failed = append(failed, t)
		} else {
			out[t] = results[i].bars
		}
	}
	return out, failed
}
