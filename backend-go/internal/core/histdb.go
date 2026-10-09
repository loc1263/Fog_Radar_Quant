package core

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	_ "modernc.org/sqlite" // driver sin CGO
)

// HistDBName es el nombre de la base histórica de cada módulo (data/<modulo>/history.db).
const HistDBName = "history.db"

const busyTimeoutMS = 30_000

// Fila es una fila de resultado: columna → valor (nil, int64, float64, string o []byte).
type Fila map[string]any

// Str / Int / Float leen una columna con la conversión que Python haría implícita.
func (f Fila) Str(col string) *string {
	switch v := f[col].(type) {
	case string:
		return &v
	case []byte:
		s := string(v)
		return &s
	}
	return nil
}

func (f Fila) Int(col string) *int64 {
	switch v := f[col].(type) {
	case int64:
		return &v
	case float64:
		n := int64(v)
		return &n
	}
	return nil
}

func (f Fila) Float(col string) *float64 {
	switch v := f[col].(type) {
	case float64:
		return &v
	case int64:
		x := float64(v)
		return &x
	}
	return nil
}

// OpenRead abre la base en solo lectura (mode=ro). Devuelve nil si no existe.
// El llamador cierra la base.
func OpenRead(path string) (*sql.DB, error) {
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		return nil, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(abs), RawQuery: "mode=ro&_pragma=busy_timeout(5000)"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	return db, nil
}

// Filas ejecuta una consulta de lectura. Devuelve [] si la base no existe y un
// 503 invalid_data si está corrupta o el esquema no calza.
func Filas(path, query string, args ...any) ([]Fila, *APIError) {
	db, err := OpenRead(path)
	if err != nil {
		return nil, invalidData(path, err)
	}
	if db == nil {
		return []Fila{}, nil
	}
	defer db.Close()
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, invalidData(path, err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, invalidData(path, err)
	}
	out := []Fila{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, invalidData(path, err)
		}
		f := make(Fila, len(cols))
		for i, c := range cols {
			f[c] = vals[i]
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, invalidData(path, err)
	}
	return out, nil
}

func invalidData(path string, err error) *APIError {
	return Err(http.StatusServiceUnavailable, "invalid_data",
		fmt.Sprintf("%s no se pudo leer: %v", filepath.Base(path), err))
}

// OpenWrite abre la base para escribir: claves foráneas (ON DELETE CASCADE), WAL y busy_timeout.
func OpenWrite(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	q := "_pragma=foreign_keys(1)&_pragma=busy_timeout(" + strconv.Itoa(busyTimeoutMS) + ")&_pragma=journal_mode(WAL)"
	u := url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(abs), RawQuery: q}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// SchemaVersion es PRAGMA user_version.
func SchemaVersion(db *sql.DB) (int, error) {
	var v int
	err := db.QueryRow("PRAGMA user_version").Scan(&v)
	return v, err
}

// Migrar aplica las migraciones pendientes según PRAGMA user_version.
//
// migraciones[i] lleva la base de la versión i a la i+1, cada una en su propia
// transacción junto con el cambio de versión. Nunca se edita una migración ya
// publicada (las mismas listas que usa Python): un cambio de esquema es una
// migración nueva al final. Devuelve la versión final.
func Migrar(db *sql.DB, migraciones []string) (int, error) {
	actual, err := SchemaVersion(db)
	if err != nil {
		return 0, err
	}
	if actual > len(migraciones) {
		return 0, fmt.Errorf("la base está en la versión %d y este código conoce hasta la %d",
			actual, len(migraciones))
	}
	for i := actual; i < len(migraciones); i++ {
		script := fmt.Sprintf("BEGIN;\n%s\nPRAGMA user_version = %d;\nCOMMIT;", migraciones[i], i+1)
		if _, err := db.Exec(script); err != nil {
			_, _ = db.Exec("ROLLBACK")
			return 0, fmt.Errorf("migración %d: %w", i+1, err)
		}
	}
	return len(migraciones), nil
}

// HistDiffFila es un ticker comparado entre dos corridas (ver HistDiffFila en Python).
type HistDiffFila struct {
	Ticker     string   `json:"ticker"`
	Nombre     *string  `json:"nombre" nullable:"true"`
	RankDesde  *int64   `json:"rank_desde" nullable:"true"`
	RankHasta  *int64   `json:"rank_hasta" nullable:"true"`
	ScoreDesde *float64 `json:"score_desde" nullable:"true"`
	ScoreHasta *float64 `json:"score_hasta" nullable:"true"`
	DeltaRank  *int64   `json:"delta_rank" nullable:"true" doc:"Positivo = subió posiciones."`
	DeltaScore *float64 `json:"delta_score" nullable:"true"`
	Estado     string   `json:"estado" enum:"nuevo,salio,sigue,fuera"`
}

// Posicion es una fila de ranking para comparar (ticker, nombre, rank, score).
type Posicion struct {
	Ticker string
	Nombre *string
	Rank   *int64
	Score  *float64
}

// DiffCorridas compara dos rankings. Con topN el estado se mide contra el top
// (nuevo/salio/sigue/fuera); sin él, contra la presencia en el ranking. Orden:
// los del top de «hasta» primero (por rank), luego los que salieron, luego el resto.
func DiffCorridas(desde, hasta []Posicion, topN *int64) []HistDiffFila {
	a := map[string]Posicion{}
	for _, p := range desde {
		a[p.Ticker] = p
	}
	b := map[string]Posicion{}
	for _, p := range hasta {
		b[p.Ticker] = p
	}
	en := func(p *Posicion) bool {
		if p == nil || p.Rank == nil {
			return p != nil && topN == nil
		}
		return topN == nil || *p.Rank <= *topN
	}

	tickers := map[string]bool{}
	for t := range a {
		tickers[t] = true
	}
	for t := range b {
		tickers[t] = true
	}
	out := make([]HistDiffFila, 0, len(tickers))
	for t := range tickers {
		var fa, fb *Posicion
		if p, ok := a[t]; ok {
			fa = &p
		}
		if p, ok := b[t]; ok {
			fb = &p
		}
		ena, enb := en(fa), en(fb)
		estado := "fuera"
		switch {
		case ena && enb:
			estado = "sigue"
		case enb:
			estado = "nuevo"
		case ena:
			estado = "salio"
		}
		row := HistDiffFila{Ticker: t, Estado: estado}
		if fb != nil {
			row.Nombre = fb.Nombre
		} else if fa != nil {
			row.Nombre = fa.Nombre
		}
		if fa != nil {
			row.RankDesde, row.ScoreDesde = fa.Rank, fa.Score
		}
		if fb != nil {
			row.RankHasta, row.ScoreHasta = fb.Rank, fb.Score
		}
		if row.RankDesde != nil && row.RankHasta != nil {
			d := *row.RankDesde - *row.RankHasta
			row.DeltaRank = &d
		}
		if row.ScoreDesde != nil && row.ScoreHasta != nil {
			d := round6(*row.ScoreHasta - *row.ScoreDesde)
			row.DeltaScore = &d
		}
		out = append(out, row)
	}

	const sinRank = int64(1_000_000_000)
	orden := map[string]int{"sigue": 0, "nuevo": 0, "salio": 1, "fuera": 2}
	rk := func(p *int64) int64 {
		if p == nil {
			return sinRank
		}
		return *p
	}
	sort.Slice(out, func(i, j int) bool {
		x, y := out[i], out[j]
		if orden[x.Estado] != orden[y.Estado] {
			return orden[x.Estado] < orden[y.Estado]
		}
		kx, ky := rk(x.RankHasta), rk(y.RankHasta)
		if x.Estado == "salio" {
			kx = sinRank
		}
		if y.Estado == "salio" {
			ky = sinRank
		}
		if kx != ky {
			return kx < ky
		}
		if rx, ry := rk(x.RankDesde), rk(y.RankDesde); rx != ry {
			return rx < ry
		}
		return x.Ticker < y.Ticker
	})
	return out
}

// round6 redondea a 6 decimales como round(x, 6) de Python (correctamente redondeado).
func round6(x float64) float64 {
	v, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 6, 64), 64)
	return v
}

// HistDiff compara dos corridas (modelo compartido por todos los módulos, como HistDiff de Python).
type HistDiff struct {
	Desde string         `json:"desde"`
	Hasta string         `json:"hasta"`
	TopN  *int64         `json:"top_n" nullable:"true"`
	Filas []HistDiffFila `json:"filas"`
}
