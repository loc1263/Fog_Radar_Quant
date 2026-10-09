package tend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fogradar/internal/jobs"
	"fogradar/internal/yahoo"
)

func init() {
	jobs.Register(string(key), jobs.Pipeline{
		Run: Run,
		Options: []jobs.Option{
			{Flag: "--skip-fetch", Help: "Reutiliza los precios/FX ya descargados (sin Yahoo)."},
			{Flag: "--force-refresh", Help: "Ignora la caché de precios y vuelve a descargar todo."},
			{Flag: "--force-discovery", Help: "Ignora el snapshot de universo de hoy."},
			{Flag: "--static-universe", Help: "Usa la lista fija de respaldo en vez del screener."},
			{Flag: "--skip-discovery", Help: "Reutiliza el último snapshot de universo (sin screener)."},
			{Flag: "--macro", Help: "Descarga antes los datos macro de FRED (requiere FRED_API_KEY)."},
			jobs.Backfill,
			{Flag: "--top-n", Help: "Acciones a seleccionar en Asia + EEUU (por defecto 15).", Kind: jobs.Int, Min: 2, Max: 60},
			{Flag: "--top-n-us", Help: "Acciones a seleccionar en el análisis solo EEUU (por defecto 15).", Kind: jobs.Int, Min: 2, Max: 60},
			{Flag: "--max-weight", Help: "Tope de peso por activo, fracción en (0, 1] (por defecto 0.15).", Kind: jobs.Float, Min: 0.01, Max: 1.0},
		},
	})
}

// referenceTickers son los tickers de referencia que se descargan junto al universo.
var referenceTickers = []string{"^VIX", "^IRX"}

func flag(opts map[string]any, k string) bool { b, _ := opts[k].(bool); return b }

// SaveSnapshot guarda el universo como data/tendencia/universe/universe_<YYYYMMDD>.json (formato de discovery.save_snapshot).
func SaveSnapshot(moduleDir string, u *Universe, method string, now time.Time) (string, error) {
	tickers := func(list []string) OM {
		o := OM{}
		for _, t := range list {
			i := u.Info[t]
			o = append(o, KV{t, []string{i.Name, i.Country, i.Currency}})
		}
		return o
	}
	sectors := OM{}
	for _, t := range u.All() {
		if s, ok := u.Sector[t]; ok {
			sectors = append(sectors, KV{t, s})
		}
	}
	dir := filepath.Join(moduleDir, "universe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "universe_"+now.Format("20060102")+".json")
	snap := OM{{"discovered_at", now.Format("2006-01-02T15:04:05.000000")}, {"method", method},
		{"asia_tickers", tickers(u.Asia)}, {"us_tickers", tickers(u.US)}, {"sector_map", sectors}}
	return path, os.WriteFile(path, PyJSON(snap), 0o644)
}

// resolveUniverse decide el universo de la corrida. El screener de Yahoo (EquityQuery) no está portado a Go: sin
// --static-universe se reutiliza el snapshot de hoy o, si no hay, el más reciente (como --skip-discovery); sin ninguno,
// la lista fija de respaldo (que se guarda como snapshot).
func resolveUniverse(moduleDir string, opts map[string]any, now time.Time, warn func(string, ...any)) (*Universe, error) {
	if flag(opts, "static_universe") {
		warn("--static-universe activo: se usa la lista fija de respaldo, sin consultar el screener.")
		u := FallbackUniverse()
		u.Source = "fallback_static_forced"
		if _, err := SaveSnapshot(moduleDir, u, u.Source, now); err != nil {
			return nil, err
		}
		return u, nil
	}
	dir := filepath.Join(moduleDir, "universe")
	today := filepath.Join(dir, "universe_"+now.Format("20060102")+".json")
	if !flag(opts, "skip_discovery") && !isFile(today) {
		warn("El descubrimiento por screener no está disponible en el backend Go: se reutiliza el último snapshot de universo (corre el screener con el backend Python si necesitas refrescarlo).")
	}
	if p := LatestUniverseFile(dir, now); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			if u, err := ParseUniverse(b); err == nil {
				warn("Universo activo: %d Asia + %d EEUU (fuente: %s, %s)", len(u.Asia), len(u.US), u.Source, filepath.Base(p))
				return u, nil
			}
		}
		warn("El snapshot %s está dañado; se usa la lista fija de respaldo.", filepath.Base(p))
	}
	u := FallbackUniverse()
	if _, err := SaveSnapshot(moduleDir, u, "fallback_static", now); err != nil {
		return nil, err
	}
	return u, nil
}

func fetchOptions(opts map[string]any) yahoo.HistoryOptions {
	o := yahoo.DefaultHistoryOptions()
	o.Range, o.Interval = "5y", "1wk"
	return o
}

// Run es el pipeline batch de Tendencia (pasos 0-8 de pipeline.py).
//
//	fogradar job tendencia [--skip-fetch] [--force-refresh] [--static-universe] [--macro] [--top-n N] [--max-weight W]
//	fogradar job tendencia --backfill
//
// Nunca se ejecuta dentro de un request web. Devuelve un resumen para el registro de jobs.
func Run(ctx context.Context, env *jobs.Env, opts map[string]any) (string, error) {
	s := env.S
	warn := func(f string, a ...any) { env.Log.Warn(fmt.Sprintf(f, a...)) }
	info := func(f string, a ...any) { env.Log.Info(fmt.Sprintf(f, a...)) }
	moduleDir := ModuleDir(s)
	now := time.Now().In(s.Loc)

	p := DefaultParams()
	overrides := OM{}
	if v, ok := opts["top_n"].(int); ok {
		p.TopN = v
		overrides = append(overrides, KV{"TOP_N_SELECTION", int64(v)})
	}
	if v, ok := opts["top_n_us"].(int); ok {
		p.TopNUS = v
		overrides = append(overrides, KV{"TOP_N_SELECTION_US", int64(v)})
	}
	if v, ok := opts["max_weight"].(float64); ok {
		p.MaxWeight = v
		overrides = append(overrides, KV{"MAX_WEIGHT_PER_ASSET", v})
	}

	if flag(opts, "backfill") {
		u := LoadUniverse(moduleDir, now)
		return Backfill(s, u, p, warn)
	}

	var partes []string
	if flag(opts, "macro") {
		out, err := FetchMacro(ctx, filepath.Join(moduleDir, "macro"), MacroOpts{APIKey: s.FredAPIKey, Yahoo: env.Yahoo, Now: now, Log: warn})
		if err != nil {
			return "", err
		}
		partes = append(partes, "macro "+filepath.Base(out))
	}

	u, err := resolveUniverse(moduleDir, opts, now, warn)
	if err != nil {
		return "", err
	}
	if problemas := Validar(u, p); len(problemas) > 0 {
		return "", errors.New("Configuración de Tendencia no factible: " + strings.Join(problemas, " "))
	}

	runDir, err := NewRunDir(s, now)
	if err != nil {
		return "", err
	}
	info("Carpeta de esta corrida: %s", runDir)

	if !flag(opts, "skip_fetch") {
		ho := fetchOptions(opts)
		force := flag(opts, "force_refresh")
		all := append(u.All(), referenceTickers...)
		info("Descarga de precios: %d tickers (Asia %d, EEUU %d, referencia %d)", len(all), len(u.Asia), len(u.US), len(referenceTickers))
		client, ok := env.Yahoo.(*yahoo.Client)
		if !ok {
			return "", errors.New("el cliente de Yahoo no admite descargas de histórico")
		}
		_, failed := client.DownloadMany(ctx, all, filepath.Join(moduleDir, "raw", "prices"), ho, force, s.YahooConcurrencia, nil)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if len(failed) > 0 {
			warn("Tickers que fallaron tras %d reintentos: %s (los descargados quedan en caché; se puede reintentar).", ho.MaxRetries, strings.Join(failed, ", "))
		}
		fxSet := map[string]bool{}
		var fxTickers []string
		for _, t := range u.Asia {
			if c := u.Info[t].Currency; c != "USD" {
				if ft := FXTicker(c); ft != "" && !fxSet[ft] {
					fxSet[ft] = true
					fxTickers = append(fxTickers, ft)
				}
			}
		}
		sort.Strings(fxTickers)
		info("Pares FX a descargar: %s", strings.Join(fxTickers, ", "))
		_, failedFX := client.DownloadMany(ctx, fxTickers, filepath.Join(moduleDir, "raw", "fx"), ho, force, s.YahooConcurrencia, nil)
		if len(failedFX) > 0 {
			warn("Pares FX que fallaron: %s. Sin estos pares, los tickers en esa moneda se excluirán del dataset.", strings.Join(failedFX, ", "))
		}
	} else {
		info("--skip-fetch activo: se reutiliza raw/ existente.")
	}

	prices, rf, err := BuildDataset(moduleDir, u, p, warn)
	if err != nil {
		return "", err
	}

	for _, spec := range Analyses(u, p) {
		if _, err := RunSelection(prices, rf, runDir, u, spec, p, warn); err != nil {
			if spec.Key == "" {
				return "", err
			}
			warn("Análisis '%s' omitido: %v", spec.Label, err)
		}
	}
	if _, err := RunForecast(prices, runDir, moduleDir, u, p, warn); err != nil {
		return "", err
	}
	if err := RunMomentum(prices, runDir, u, p, warn); err != nil {
		return "", err
	}
	if _, err := GenerarPDF(runDir, u, p, LatestMacroFile(moduleDir) != "", now); err != nil {
		warn("No se pudo generar el informe PDF (%v); dashboard_data.json y los CSV sí se generan.", err)
	}

	payload, err := BuildDashboardPayload(runDir, u, p, now)
	if err != nil {
		return "", err
	}
	if err := WriteDashboard(runDir, payload); err != nil {
		return "", err
	}
	if err := writeManifest(runDir, now, p, u, overrides); err != nil {
		warn("No se pudo registrar run_manifest.json: %v", err)
	}
	// El índice histórico es derivado: si falla se avisa, pero la corrida no falla (se reconstruye con --backfill).
	if err := StoreRun(s, payload, runDir); err != nil {
		warn("la corrida no se guardó en history.db; se recupera con: fogradar job tendencia --backfill (%v)", err)
	}

	partes = append([]string{"corrida " + filepath.Base(runDir)}, partes...)
	for _, a := range asList(payload.Get("analyses")) {
		an := a.(OM)
		perf, _ := an.Get("performance").(OM)
		sharpe, _ := perfMetric(perf, "portafolio_15_finales", "sharpe_ratio").(float64)
		var top []string
		for i, h := range asList(an.Get("holdings")) {
			if i >= 3 {
				break
			}
			top = append(top, h.(OM).Get("ticker").(string))
		}
		partes = append(partes, fmt.Sprintf("%s: Sharpe %.2f, top %s", an.Get("label"), sharpe, strings.Join(top, ", ")))
	}
	return strings.Join(partes, " · "), nil
}

// writeManifest deja en run_manifest.json los parámetros efectivos (reproducibilidad).
func writeManifest(runDir string, now time.Time, p Params, u *Universe, overrides OM) error {
	path := filepath.Join(runDir, "run_manifest.json")
	manifest := OM{{"created_at", now.Format("2006-01-02T15:04:05.000000")}}
	if b, err := os.ReadFile(path); err == nil {
		if v, err := ParseOM(b); err == nil {
			if om, ok := v.(OM); ok {
				manifest = om
			}
		}
	}
	set := func(k string, v any) {
		for i := range manifest {
			if manifest[i].K == k {
				manifest[i].V = v
				return
			}
		}
		manifest = append(manifest, KV{k, v})
	}
	set("parametros", OM{{"top_n", int64(p.TopN)}, {"top_n_us", int64(p.TopNUS)}, {"max_weight_per_asset", p.MaxWeight},
		{"l2_gamma", p.L2Gamma}, {"min_weeks_required", int64(p.MinWeeks)}, {"universe_source", u.Source}})
	if len(overrides) > 0 {
		set("parametros_sobrescritos", overrides)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, PyJSON(manifest), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
