package us

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fogradar/internal/core"
)

// SchemaV1 de history.db (idéntico a SCHEMA_V1 de Python: ambas implementaciones comparten la base).
const SchemaV1 = `
CREATE TABLE runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT UNIQUE NOT NULL,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    status TEXT NOT NULL,
    min_market_cap REAL,
    universe_total INTEGER,
    candidates_total INTEGER,
    evaluated_total INTEGER,
    top_n INTEGER NOT NULL,
    stored_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE candidates (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    rank INTEGER NOT NULL,
    in_top INTEGER NOT NULL,
    name TEXT,
    sector TEXT,
    market_cap REAL,
    price REAL,
    momentum_12_1 REAL,
    return_1d REAL,
    return_5d REAL,
    return_3m REAL,
    return_6m REAL,
    return_1y REAL,
    return_ytd REAL,
    rsi_14 REAL,
    sma_50 REAL,
    sma_200 REAL,
    score REAL,
    score_label TEXT,
    valuation REAL,
    profitability REAL,
    financial_health REAL,
    shareholder_return REAL,
    technical REAL,
    PRIMARY KEY (run_pk, ticker)
);
CREATE INDEX idx_candidates_ticker ON candidates(ticker);

CREATE TABLE prices (
    ticker TEXT NOT NULL,
    date TEXT NOT NULL,
    close REAL NOT NULL,
    PRIMARY KEY (ticker, date)
) WITHOUT ROWID;
`

// Migraciones de history.db.
var Migraciones = []string{SchemaV1}

func dbPath(s *core.Settings) string { return filepath.Join(s.ModuleDataDir(key), core.HistDBName) }

// --- Escritura ---

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// guardarCorrida guarda la corrida; false si no está terminada (sin result.json o con error).
func guardarCorrida(tx execer, runDir string) (bool, error) {
	var result RunResponse
	if e := ReadRunFile(runDir, "result.json", &result); e != nil {
		return false, nil
	}
	if result.Status != "done" {
		return false, nil
	}
	cand := result
	if e := ReadRunFile(runDir, "candidates.json", &cand); e != nil {
		cand = result
	}
	topN := result.TopN
	if topN == 0 {
		topN = 15
	}
	// started_at / finished_at tal como están en el archivo (texto)
	raw := map[string]any{}
	if b, err := os.ReadFile(filepath.Join(runDir, "result.json")); err == nil {
		_ = json.Unmarshal(b, &raw)
	}
	if _, err := tx.Exec("DELETE FROM runs WHERE run_id = ?", result.RunID); err != nil {
		return false, err
	}
	res, err := tx.Exec(`INSERT INTO runs (run_id, started_at, finished_at, status, min_market_cap, universe_total,
		candidates_total, evaluated_total, top_n) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		result.RunID, raw["started_at"], raw["finished_at"], result.Status, result.MinMarketCap,
		result.UniverseTotal, result.CandidatesTotal, len(cand.Rows), topN)
	if err != nil {
		return false, err
	}
	runPK, err := res.LastInsertId()
	if err != nil {
		return false, err
	}
	for i, r := range cand.Rows {
		rank := i + 1
		inTop := 0
		if int64(rank) <= topN {
			inTop = 1
		}
		_, err := tx.Exec(`INSERT INTO candidates (run_pk, ticker, rank, in_top, name, sector, market_cap, price, momentum_12_1,
			return_1d, return_5d, return_3m, return_6m, return_1y, return_ytd, rsi_14, sma_50, sma_200, score, score_label,
			valuation, profitability, financial_health, shareholder_return, technical)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			runPK, r.Ticker, rank, inTop, r.Name, r.Sector, r.MarketCap, r.Price, r.Momentum121,
			r.Return1d, r.Return5d, r.Return3m, r.Return6m, r.Return1y, r.ReturnYtd,
			r.Technical.Rsi14, r.Technical.Sma50, r.Technical.Sma200, r.Score.Overall, nullIfEmpty(r.Score.Label),
			r.Score.Categories.Valuation, r.Score.Categories.Profitability, r.Score.Categories.FinancialHealth,
			r.Score.Categories.ShareholderReturn, r.Score.Categories.Technical)
		if err != nil {
			return false, err
		}
	}
	var m PriceMatrix
	if e := ReadRunFile(runDir, "prices.json", &m); e == nil {
		// La corrida más nueva manda (Yahoo ajusta cierres viejos por dividendos/splits).
		for t, serie := range m.Prices {
			for i, c := range serie {
				if c == nil || i >= len(m.Dates) {
					continue
				}
				if _, err := tx.Exec(`INSERT INTO prices (ticker, date, close) VALUES (?, ?, ?)
					ON CONFLICT(ticker, date) DO UPDATE SET close = excluded.close`, t, m.Dates[i], *c); err != nil {
					return false, err
				}
			}
		}
	}
	return true, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// GuardarCorrida guarda (o reemplaza) una corrida terminada en history.db.
func GuardarCorrida(s *core.Settings, runDir string) error {
	db, err := core.OpenWrite(dbPath(s))
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := core.Migrar(db, Migraciones); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := guardarCorrida(tx, runDir); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Backfill carga todas las corridas terminadas de runs/ (idempotente), de la más antigua a la más
// nueva (así los cierres quedan con el último ajuste).
func Backfill(s *core.Settings, warn func(format string, args ...any)) (string, error) {
	dirs := ListRuns(s)
	if len(dirs) == 0 {
		return fmt.Sprintf("backfill: no hay corridas en %s", RunsDir(s)), nil
	}
	sort.Slice(dirs, func(i, j int) bool { return filepath.Base(dirs[i]) < filepath.Base(dirs[j]) })
	db, err := core.OpenWrite(dbPath(s))
	if err != nil {
		return "", err
	}
	defer db.Close()
	if _, err := core.Migrar(db, Migraciones); err != nil {
		return "", err
	}
	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	ok, omitidas := 0, []string{}
	for _, d := range dirs {
		if _, err := tx.Exec("SAVEPOINT corrida"); err != nil {
			_ = tx.Rollback()
			return "", err
		}
		guardada, gerr := guardarCorrida(tx, d)
		if gerr != nil {
			if _, rerr := tx.Exec("ROLLBACK TO corrida"); rerr != nil {
				_ = tx.Rollback()
				return "", rerr
			}
			if warn != nil {
				warn("backfill: se omite %s (%v)", filepath.Base(d), gerr)
			}
			guardada = false
		}
		_, _ = tx.Exec("RELEASE corrida")
		if guardada {
			ok++
		} else {
			omitidas = append(omitidas, filepath.Base(d))
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	res := fmt.Sprintf("backfill: %d corridas cargadas en history.db", ok)
	if len(omitidas) > 0 {
		res += fmt.Sprintf(", %d omitidas (sin terminar o con error: %s)", len(omitidas), strings.Join(omitidas, ", "))
	}
	return res, nil
}

// --- Lectura ---

func fechaDe(f core.Fila, col string, s *core.Settings) (core.Fecha, *core.APIError) {
	raw := f.Str(col)
	if raw == nil {
		return core.Fecha{}, nil
	}
	t, err := core.ParseISO(*raw, s.Loc)
	if err != nil {
		return core.Fecha{}, core.Err(http.StatusServiceUnavailable, "invalid_data", "history.db: "+err.Error())
	}
	return core.Fecha{Time: t}, nil
}

// Corridas lista las corridas guardadas.
func Corridas(s *core.Settings) ([]UsHistCorrida, *core.APIError) {
	rows, e := core.Filas(dbPath(s), `
		SELECT r.run_id, r.started_at, r.finished_at, r.universe_total, r.candidates_total,
		       r.evaluated_total, r.top_n, c.ticker AS top1_ticker, c.score AS top1_score
		FROM runs r
		LEFT JOIN candidates c ON c.run_pk = r.id AND c.rank = 1
		ORDER BY r.run_id ASC`)
	if e != nil {
		return nil, e
	}
	out := make([]UsHistCorrida, 0, len(rows))
	for _, f := range rows {
		st, e := fechaDe(f, "started_at", s)
		if e != nil {
			return nil, e
		}
		h := UsHistCorrida{RunID: str(f, "run_id"), StartedAt: st, UniverseTotal: f.Int("universe_total"),
			CandidatesTotal: f.Int("candidates_total"), EvaluatedTotal: f.Int("evaluated_total"),
			TopN: deref(f.Int("top_n")), Top1Ticker: f.Str("top1_ticker"), Top1Score: f.Float("top1_score")}
		if f.Str("finished_at") != nil {
			fin, e := fechaDe(f, "finished_at", s)
			if e != nil {
				return nil, e
			}
			h.FinishedAt = &fin
		}
		out = append(out, h)
	}
	return out, nil
}

func str(f core.Fila, c string) string {
	if p := f.Str(c); p != nil {
		return *p
	}
	return ""
}

func deref(n *int64) int64 {
	if n == nil {
		return 0
	}
	return *n
}

// HistoriaTicker es la evolución de un ticker en las corridas.
func HistoriaTicker(s *core.Settings, ticker string) ([]UsHistTickerPunto, *core.APIError) {
	rows, e := core.Filas(dbPath(s), `
		SELECT r.run_id, r.started_at, r.evaluated_total, r.top_n,
		       c.rank, c.in_top, c.name, c.price, c.momentum_12_1, c.score, c.score_label,
		       c.valuation, c.profitability, c.financial_health, c.shareholder_return, c.technical
		FROM candidates c
		JOIN runs r ON r.id = c.run_pk
		WHERE c.ticker = ?
		ORDER BY r.run_id ASC`, ticker)
	if e != nil {
		return nil, e
	}
	if len(rows) == 0 {
		return nil, core.Err(http.StatusNotFound, "not_found", fmt.Sprintf("'%s' no aparece entre los candidatos del historial.", ticker))
	}
	out := make([]UsHistTickerPunto, 0, len(rows))
	for _, f := range rows {
		st, e := fechaDe(f, "started_at", s)
		if e != nil {
			return nil, e
		}
		out = append(out, UsHistTickerPunto{RunID: str(f, "run_id"), StartedAt: st, EvaluatedTotal: f.Int("evaluated_total"),
			TopN: deref(f.Int("top_n")), Rank: deref(f.Int("rank")), InTop: deref(f.Int("in_top")) != 0,
			Name: f.Str("name"), Price: f.Float("price"), Momentum121: f.Float("momentum_12_1"),
			Score: f.Float("score"), ScoreLabel: f.Str("score_label"),
			Pilares: UsHistPilares{Valuation: f.Float("valuation"), Profitability: f.Float("profitability"),
				FinancialHealth: f.Float("financial_health"), ShareholderReturn: f.Float("shareholder_return"),
				Technical: f.Float("technical")}})
	}
	return out, nil
}

// PermanenciaTop: cuántas de las últimas «ultimas» corridas estuvo cada ticker en el top y su racha actual.
func PermanenciaTop(s *core.Settings, ultimas int) ([]UsHistPermanencia, *core.APIError) {
	rows, e := core.Filas(dbPath(s), `
		WITH ult AS (SELECT id, run_id FROM runs ORDER BY run_id DESC LIMIT ?)
		SELECT u.run_id, c.ticker, c.name, c.rank, c.in_top, c.score
		FROM candidates c JOIN ult u ON u.id = c.run_pk
		ORDER BY u.run_id DESC`, ultimas)
	if e != nil {
		return nil, e
	}
	if len(rows) == 0 {
		return []UsHistPermanencia{}, nil
	}
	runSet := map[string]bool{}
	for _, f := range rows {
		runSet[str(f, "run_id")] = true
	}
	runs := make([]string, 0, len(runSet))
	for r := range runSet {
		runs = append(runs, r)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(runs)))

	type acc struct {
		ticker string
		nombre *string
		veces  int64
		enTop  map[string]bool
		ultimo core.Fila
	}
	by := map[string]*acc{}
	var order []string
	for _, f := range rows {
		t := str(f, "ticker")
		d, ok := by[t]
		if !ok {
			d = &acc{ticker: t, nombre: f.Str("name"), enTop: map[string]bool{}}
			by[t] = d
			order = append(order, t)
		}
		if deref(f.Int("in_top")) != 0 {
			d.veces++
			d.enTop[str(f, "run_id")] = true
		}
		if d.ultimo == nil {
			d.ultimo = f
		}
	}
	out := []UsHistPermanencia{}
	for _, t := range order {
		d := by[t]
		if d.veces == 0 {
			continue
		}
		var racha int64
		for _, r := range runs {
			if !d.enTop[r] {
				break
			}
			racha++
		}
		h := UsHistPermanencia{Ticker: d.ticker, Nombre: d.nombre, VecesEnTop: d.veces, Corridas: int64(len(runs)), RachaActual: racha}
		if str(d.ultimo, "run_id") == runs[0] {
			h.RankUltima, h.ScoreUltima = d.ultimo.Int("rank"), d.ultimo.Float("score")
		}
		out = append(out, h)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.VecesEnTop != b.VecesEnTop {
			return a.VecesEnTop > b.VecesEnTop
		}
		if a.RachaActual != b.RachaActual {
			return a.RachaActual > b.RachaActual
		}
		return a.Ticker < b.Ticker
	})
	return out, nil
}

func rankingDe(s *core.Settings, runID string) ([]core.Posicion, *core.APIError) {
	rows, e := core.Filas(dbPath(s), `
		SELECT c.ticker, c.name AS nombre, c.rank, c.score
		FROM candidates c JOIN runs r ON r.id = c.run_pk
		WHERE r.run_id = ?`, runID)
	if e != nil {
		return nil, e
	}
	out := make([]core.Posicion, 0, len(rows))
	for _, f := range rows {
		out = append(out, core.Posicion{Ticker: str(f, "ticker"), Nombre: f.Str("nombre"), Rank: f.Int("rank"), Score: f.Float("score")})
	}
	return out, nil
}

// Diff: qué entró, salió o cambió en el top entre dos corridas.
func Diff(s *core.Settings, desde, hasta string) (*core.HistDiff, *core.APIError) {
	cs, e := Corridas(s)
	if e != nil {
		return nil, e
	}
	ids := make([]string, len(cs))
	tops := map[string]int64{}
	for i, c := range cs {
		ids[i] = c.RunID
		tops[c.RunID] = c.TopN
	}
	switch {
	case desde == "" && hasta == "":
		if len(ids) < 2 {
			return nil, core.Err(http.StatusNotFound, "not_found", "Hacen falta al menos dos corridas en el historial.")
		}
		desde, hasta = ids[len(ids)-2], ids[len(ids)-1]
	case desde == "" || hasta == "":
		return nil, core.Err(http.StatusUnprocessableEntity, "validation_error", "Indica 'desde' y 'hasta', o ninguno de los dos.")
	}
	for _, id := range []string{desde, hasta} {
		if _, ok := tops[id]; !ok {
			return nil, core.Err(http.StatusNotFound, "not_found", fmt.Sprintf("No existe la corrida '%s' en el historial.", id))
		}
	}
	a, e := rankingDe(s, desde)
	if e != nil {
		return nil, e
	}
	b, e := rankingDe(s, hasta)
	if e != nil {
		return nil, e
	}
	topN := tops[hasta]
	return &core.HistDiff{Desde: desde, Hasta: hasta, TopN: &topN, Filas: core.DiffCorridas(a, b, &topN)}, nil
}
