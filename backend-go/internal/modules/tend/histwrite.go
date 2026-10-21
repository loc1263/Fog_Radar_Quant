package tend

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"fogradar/internal/core"
)

// Escritura de history.db. Es un índice derivado: los archivos de cada
// corrida siguen siendo la fuente de verdad y se reconstruye con --backfill.

var runDateRe = regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})_(\d{2})$`)

var (
	holdingColumns = []string{"ticker", "nombre", "pais", "sector", "peso_final", "precio_actual_usd", "senal_tecnica", "ma_10s",
		"ma_40s", "momentum_26s_pct", "retorno_esperado_anual_pct", "volatilidad_anual_pct", "precio_proyectado_6m_usd",
		"precio_min_90pct_6m_usd", "precio_max_90pct_6m_usd", "retorno_esperado_6m_pct"}
	sectorColumns   = []string{"sector", "n_acciones", "peso_total", "retorno_6m_prom", "senal_predominante", "tickers"}
	momentumColumns = []string{"momentum_rank", "ticker", "nombre", "pais", "sector", "precio_actual_usd", "momentum_1m_pct",
		"momentum_2m_pct", "momentum_3m_pct", "momentum_4m_pct", "momentum_6m_pct", "momentum_8m_pct", "momentum_10m_pct",
		"momentum_12m_pct", "pct_meses_positivos_12m", "momentum_score_pct"}
)

func runDate(runDir string) string {
	if m := runDateRe.FindStringSubmatch(runDir); m != nil {
		return m[1] + "-" + m[2] + "-" + m[3]
	}
	if len(runDir) > 8 {
		return runDir[:8]
	}
	return runDir
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func insertRows(tx execer, table string, columns []string, analysisID int64, rows []any) error {
	cols := append([]string{"analysis_id"}, columns...)
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "), strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", "))
	for _, r := range rows {
		row, _ := r.(OM)
		args := []any{analysisID}
		for _, c := range columns {
			key := c
			if c == "momentum_rank" {
				key = "rank"
			}
			args = append(args, row.Get(key))
		}
		if _, err := tx.Exec(q, args...); err != nil {
			return err
		}
	}
	return nil
}

type uwRow struct {
	rank          int
	ticker        string
	nombre, pais  any
	peso, retorno any
}

// universeWeights lee markowitz_universe_weights<key>.csv ordenado por peso descendente (estable).
func universeWeights(runPath, key string) []uwRow {
	if runPath == "" {
		return nil
	}
	f := readCSVOrEmpty(filepath.Join(runPath, "markowitz_universe_weights"+key+".csv"))
	if f == nil || f.Col("peso_markowitz") == nil {
		return nil
	}
	idx := make([]int, f.N)
	for i := range idx {
		idx[i] = i
	}
	pc := f.Col("peso_markowitz")
	sort.SliceStable(idx, func(a, b int) bool {
		x, y := pc.At(idx[a]), pc.At(idx[b])
		if math.IsNaN(x) {
			return false
		}
		return math.IsNaN(y) || x > y
	})
	val := func(name string, i int) any {
		if c := f.Col(name); c != nil {
			return c.Value(i)
		}
		return nil
	}
	var out []uwRow
	for n, i := range idx {
		out = append(out, uwRow{n + 1, f.Cols[0].Cell(i), val("nombre", i), val("pais", i), val("peso_markowitz", i), val("retorno_esperado_anual", i)})
	}
	return out
}

func asList(v any) []any { l, _ := v.([]any); return l }

func perfMetric(perf OM, group, key string) any {
	g, _ := perf.Get(group).(OM)
	return g.Get(key)
}

// storeRun guarda un payload en history.db dentro de la transacción dada (idempotente por run_dir).
func storeRun(tx execer, payload OM, runPath string) error {
	runDir, _ := payload.Get("run_dir").(string)
	if _, err := tx.Exec("DELETE FROM runs WHERE run_dir = ?", runDir); err != nil { // CASCADE borra las hijas
		return err
	}
	// separador ", ".
	var ns []string
	for _, n := range asList(payload.Get("market_regime_notes")) {
		ns = append(ns, jsonString(n.(string)))
	}
	notes := "[" + strings.Join(ns, ", ") + "]"
	res, err := tx.Exec(`INSERT INTO runs (run_dir, run_date, generated_at, universe_source, market_regime_notes) VALUES (?, ?, ?, ?, ?)`,
		runDir, runDate(runDir), payload.Get("generated_at"), payload.Get("universe_source"), notes)
	if err != nil {
		return err
	}
	runID, _ := res.LastInsertId()
	for _, a := range asList(payload.Get("analyses")) {
		an := a.(OM)
		perf, _ := an.Get("performance").(OM)
		key, _ := an.Get("key").(string)
		res, err := tx.Exec(`INSERT INTO analyses (run_id, analysis_key, label, universe_size, n_selected,
			retorno_universo_anual, volatilidad_universo_anual, sharpe_universo,
			retorno_portafolio_anual, volatilidad_portafolio_anual, sharpe_portafolio, tasa_libre_riesgo_anual)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			runID, key, an.Get("label"), an.Get("universe_size"), an.Get("n_selected"),
			perfMetric(perf, "universo_completo", "retorno_esperado_anual"), perfMetric(perf, "universo_completo", "volatilidad_anual"),
			perfMetric(perf, "universo_completo", "sharpe_ratio"),
			perfMetric(perf, "portafolio_15_finales", "retorno_esperado_anual"), perfMetric(perf, "portafolio_15_finales", "volatilidad_anual"),
			perfMetric(perf, "portafolio_15_finales", "sharpe_ratio"), perf.Get("tasa_libre_riesgo_anual"))
		if err != nil {
			return err
		}
		aid, _ := res.LastInsertId()
		if err := insertRows(tx, "holdings", holdingColumns, aid, asList(an.Get("holdings"))); err != nil {
			return err
		}
		if err := insertRows(tx, "sectors", sectorColumns, aid, asList(an.Get("sectors"))); err != nil {
			return err
		}
		if err := insertRows(tx, "momentum", momentumColumns, aid, asList(an.Get("momentum"))); err != nil {
			return err
		}
		for _, r := range universeWeights(runPath, key) {
			if _, err := tx.Exec(`INSERT INTO universe_weights (analysis_id, universe_rank, ticker, nombre, pais, peso_markowitz,
				retorno_esperado_anual) VALUES (?, ?, ?, ?, ?, ?, ?)`, aid, r.rank, r.ticker, r.nombre, r.pais, r.peso, r.retorno); err != nil {
				return err
			}
		}
	}
	return nil
}

func openHist(s *core.Settings) (*sql.DB, error) {
	db, err := core.OpenWrite(HistoryDB(s))
	if err != nil {
		return nil, err
	}
	if _, err := core.Migrar(db, Migraciones); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// StoreRun guarda (o reemplaza) la corrida del payload en history.db.
func StoreRun(s *core.Settings, payload OM, runPath string) error {
	db, err := openHist(s)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := storeRun(tx, payload, runPath); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Backfill carga en history.db las corridas de output/ (idempotente) y devuelve el resumen.
func Backfill(s *core.Settings, u *Universe, p Params, warn func(string, ...any)) (string, error) {
	ents, _ := os.ReadDir(OutputDir(s))
	var dirs []string
	for _, e := range ents {
		if e.IsDir() && runDirRe.MatchString(e.Name()) {
			dirs = append(dirs, filepath.Join(OutputDir(s), e.Name()))
		}
	}
	if len(dirs) == 0 {
		return fmt.Sprintf("No hay carpetas de corrida en %s.", OutputDir(s)), nil
	}
	sort.Strings(dirs)
	db, err := openHist(s)
	if err != nil {
		return "", err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	ok, omitidas := 0, []string{}
	for _, d := range dirs {
		name := filepath.Base(d)
		payload, err := BuildDashboardPayload(d, u, p, time.Now().In(s.Loc))
		if err != nil {
			warn("[omitida] %s: %v", name, err)
			omitidas = append(omitidas, name)
			continue
		}
		// Conserva la fecha de generación original si la corrida la tiene archivada.
		if b, err := os.ReadFile(filepath.Join(d, DashboardFile)); err == nil {
			var old struct {
				GeneratedAt string `json:"generated_at"`
			}
			if json.Unmarshal(b, &old) == nil && old.GeneratedAt != "" {
				for i := range payload {
					if payload[i].K == "generated_at" {
						payload[i].V = old.GeneratedAt
					}
				}
			}
		}
		if _, err := tx.Exec("SAVEPOINT corrida"); err != nil {
			_ = tx.Rollback()
			return "", err
		}
		if err := storeRun(tx, payload, d); err != nil {
			_, _ = tx.Exec("ROLLBACK TO corrida")
			_, _ = tx.Exec("RELEASE corrida")
			warn("[omitida] %s: %v", name, err)
			omitidas = append(omitidas, name)
			continue
		}
		_, _ = tx.Exec("RELEASE corrida")
		warn("[cargada] %s (%d análisis)", name, len(asList(payload.Get("analyses"))))
		ok++
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	resumen := fmt.Sprintf("backfill: %d corridas cargadas en %s", ok, core.HistDBName)
	if len(omitidas) > 0 {
		resumen += fmt.Sprintf(", %d omitidas (%s)", len(omitidas), strings.Join(omitidas, ", "))
	}
	return resumen, nil
}

// NewRunDir crea output/YYYYMMDD_NN con la siguiente secuencia de hoy y su run_manifest.json.
func NewRunDir(s *core.Settings, now time.Time) (string, error) {
	today := now.In(s.Loc).Format("20060102")
	last := 0
	ents, _ := os.ReadDir(OutputDir(s))
	for _, e := range ents {
		if m := runDateRe.FindStringSubmatch(e.Name()); e.IsDir() && m != nil && m[1]+m[2]+m[3] == today {
			n, _ := strconv.Atoi(m[4])
			last = max(last, n)
		}
	}
	dir := filepath.Join(OutputDir(s), fmt.Sprintf("%s_%02d", today, last+1))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	manifest := OM{{"created_at", now.In(s.Loc).Format("2006-01-02T15:04:05.000000")}}
	return dir, os.WriteFile(filepath.Join(dir, "run_manifest.json"), OrderedJSON(manifest), 0o644)
}
