package cl

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"fogradar/internal/core"
	"fogradar/internal/quant"
)

// SchemaV1 es el esquema publicado de history.db (esquema publicado: ambas
// implementaciones comparten la misma base). Nunca se edita; un cambio es una migración nueva.
const SchemaV1 = `
CREATE TABLE runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ts TEXT UNIQUE NOT NULL,
    run_at TEXT NOT NULL,
    archivo TEXT NOT NULL,
    n_tickers INTEGER NOT NULL,
    stored_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE ranking (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    rank INTEGER,
    nombre TEXT,
    sector TEXT,
    industria TEXT,
    score_compuesto REAL,
    score_valor REAL,
    score_calidad REAL,
    score_crecimiento REAL,
    score_salud_financiera REAL,
    score_dividendo REAL,
    score_momentum REAL,
    metricas_disponibles INTEGER,
    grupo_duplicado INTEGER,
    PRIMARY KEY (run_pk, ticker)
);
CREATE INDEX idx_ranking_ticker ON ranking(ticker);

-- Formato largo: agregar una métrica al extractor no requiere migrar.
CREATE TABLE indicadores (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    metrica TEXT NOT NULL,
    valor REAL,
    PRIMARY KEY (run_pk, ticker, metrica)
);
CREATE INDEX idx_indicadores_ticker ON indicadores(ticker, metrica);
`

// Migraciones de history.db, en orden.
var Migraciones = []string{SchemaV1}

// pilaresSQL: columna del CSV de ranking → columna de la tabla ranking.
var pilaresSQL = []struct{ csv, sql string }{
	{"Score Valor", "score_valor"}, {"Score Calidad", "score_calidad"}, {"Score Crecimiento", "score_crecimiento"},
	{"Score Salud Financiera", "score_salud_financiera"}, {"Score Dividendo", "score_dividendo"},
	{"Score Momentum", "score_momentum"},
}

func dbPath(s *core.Settings) string { return filepath.Join(DataDir(s), core.HistDBName) }

func tsDe(csvPath string) string {
	return strings.TrimPrefix(strings.TrimSuffix(filepath.Base(csvPath), filepath.Ext(csvPath)), "ranking_acciones_")
}

func nullFloat(c *quant.Column, i int) any {
	if c == nil {
		return nil
	}
	if f := c.At(i); f == f { // no NaN
		return f
	}
	return nil
}

func nullInt(c *quant.Column, i int) any {
	if c == nil {
		return nil
	}
	if f := c.At(i); f == f {
		return int64(f)
	}
	return nil
}

func nullStr(c *quant.Column, i int) any {
	if c == nil || c.IsNull(i) {
		return nil
	}
	return c.Cell(i)
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// guardar inserta (o reemplaza) la corrida del CSV de ranking dentro de una transacción.
func guardar(tx execer, s *core.Settings, rankingCSV string) error {
	rank, err := quant.ReadCSVFile(rankingCSV)
	if err != nil {
		return err
	}
	ts := tsDe(rankingCSV)
	runAt := ts
	if t := TimestampDe(rankingCSV, s.Loc); t != nil {
		runAt = t.Format("2006-01-02T15:04:05-07:00")
	}
	if _, err := tx.Exec("DELETE FROM runs WHERE ts = ?", ts); err != nil { // idempotente (CASCADE borra las hijas)
		return err
	}
	res, err := tx.Exec("INSERT INTO runs (ts, run_at, archivo, n_tickers) VALUES (?, ?, ?, ?)",
		ts, runAt, filepath.Base(rankingCSV), rank.N)
	if err != nil {
		return err
	}
	runPK, err := res.LastInsertId()
	if err != nil {
		return err
	}

	sym := rank.Col("Símbolo")
	cols := map[string]*quant.Column{}
	for _, n := range []string{"Ranking", "Nombre corto", "Sector", "Industria", "Score Compuesto",
		"Métricas Disponibles", "Grupo Financiero Duplicado"} {
		cols[n] = rank.Col(n)
	}
	for _, p := range pilaresSQL {
		cols[p.csv] = rank.Col(p.csv)
	}
	for i := 0; i < rank.N; i++ {
		args := []any{runPK, sym.Cell(i), nullInt(cols["Ranking"], i), nullStr(cols["Nombre corto"], i),
			nullStr(cols["Sector"], i), nullStr(cols["Industria"], i), nullFloat(cols["Score Compuesto"], i)}
		for _, p := range pilaresSQL {
			args = append(args, nullFloat(cols[p.csv], i))
		}
		args = append(args, nullInt(cols["Métricas Disponibles"], i), nullInt(cols["Grupo Financiero Duplicado"], i))
		if _, err := tx.Exec(`INSERT INTO ranking (run_pk, ticker, rank, nombre, sector, industria, score_compuesto,
			score_valor, score_calidad, score_crecimiento, score_salud_financiera, score_dividendo, score_momentum,
			metricas_disponibles, grupo_duplicado) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, args...); err != nil {
			return err
		}
	}

	indCSV := filepath.Join(filepath.Dir(rankingCSV), "indicadores_acciones_"+ts+".csv")
	if ind, err := quant.ReadCSVFile(indCSV); err == nil && ind.Has("Símbolo") {
		isym := ind.Col("Símbolo")
		for i := 0; i < ind.N; i++ {
			for _, c := range Columnas {
				if c.Unidad == "texto" {
					continue
				}
				col := ind.Col(c.CSV)
				if col == nil {
					continue
				}
				if v := nullFloat(col, i); v != nil {
					if _, err := tx.Exec("INSERT INTO indicadores (run_pk, ticker, metrica, valor) VALUES (?,?,?,?)",
						runPK, isym.Cell(i), c.Key, v); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// GuardarCorrida guarda (o reemplaza) la corrida del CSV de ranking en history.db.
func GuardarCorrida(s *core.Settings, rankingCSV string) error {
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
	if err := guardar(tx, s, rankingCSV); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Backfill carga en history.db todos los ranking_acciones_*.csv (idempotente) y devuelve el resumen.
func Backfill(s *core.Settings, warn func(format string, args ...any)) (string, error) {
	files := ListFiles(DataDir(s), PatronRanking)
	if len(files) == 0 {
		return fmt.Sprintf("backfill: no hay %s en %s", PatronRanking, DataDir(s)), nil
	}
	// de la más antigua a la más nueva, ordenadas
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
		files[i], files[j] = files[j], files[i]
	}
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
	ok, omitidos := 0, []string{}
	for _, p := range files {
		if _, err := tx.Exec("SAVEPOINT corrida"); err != nil {
			_ = tx.Rollback()
			return "", err
		}
		if err := guardar(tx, s, p); err != nil {
			if _, rerr := tx.Exec("ROLLBACK TO corrida"); rerr != nil {
				_ = tx.Rollback()
				return "", rerr
			}
			_, _ = tx.Exec("RELEASE corrida")
			if warn != nil {
				warn("backfill: se omite %s (%v)", filepath.Base(p), err)
			}
			omitidos = append(omitidos, filepath.Base(p))
			continue
		}
		_, _ = tx.Exec("RELEASE corrida")
		ok++
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	resumen := fmt.Sprintf("backfill: %d corridas cargadas en history.db", ok)
	if len(omitidos) > 0 {
		resumen += fmt.Sprintf(", %d omitidas (%s)", len(omitidos), strings.Join(omitidos, ", "))
	}
	return resumen, nil
}
