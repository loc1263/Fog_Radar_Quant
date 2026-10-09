package etf

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"fogradar/internal/core"
	"fogradar/internal/quant"
)

// SchemaV1 de history.db (idéntico a SCHEMA_V1 de Python: ambas implementaciones comparten la base).
const SchemaV1 = `
CREATE TABLE runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_key TEXT UNIQUE NOT NULL,
    fecha TEXT NOT NULL,
    secuencia INTEGER NOT NULL,
    run_at TEXT,
    universo_bruto INTEGER,
    universo_depurado INTEGER NOT NULL,
    n_outliers INTEGER,
    sharpe_promedio REAL,
    beta_promedio REAL,
    top1_ticker TEXT,
    top1_score REAL,
    stored_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_runs_fecha ON runs(fecha, secuencia);

CREATE TABLE ranking (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    rank INTEGER NOT NULL,
    en_top15 INTEGER NOT NULL,
    nombre TEXT,
    category TEXT,
    proveedor TEXT,
    precio REAL,
    aum REAL,
    expense_ratio REAL,
    perf_5d REAL,
    perf_1m REAL,
    perf_6m REAL,
    perf_ytd REAL,
    perf_1y REAL,
    sharpe REAL,
    treynor REAL,
    beta REAL,
    s_sharpe REAL,
    s_treynor REAL,
    s_1y REAL,
    s_6m REAL,
    s_consistencia REAL,
    s_tendencia REAL,
    s_baja_dispersion REAL,
    s_bajo_beta REAL,
    s_cerca_maximo REAL,
    score REAL,
    score_simple REAL,
    PRIMARY KEY (run_pk, ticker)
);
CREATE INDEX idx_ranking_ticker ON ranking(ticker);

CREATE TABLE momentum (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    rank INTEGER NOT NULL,
    nombre TEXT,
    category TEXT,
    m_norm REAL,
    q_norm REAL,
    consist_norm REAL,
    score_comp REAL,
    PRIMARY KEY (run_pk, ticker)
);
CREATE INDEX idx_momentum_ticker ON momentum(ticker);

CREATE TABLE excluidos (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    ticker TEXT NOT NULL,
    nombre TEXT,
    category TEXT,
    ventana TEXT,
    z_score REAL,
    PRIMARY KEY (run_pk, ticker)
);
CREATE INDEX idx_excluidos_ticker ON excluidos(ticker);

CREATE TABLE categorias (
    run_pk INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    category TEXT NOT NULL,
    n INTEGER,
    sharpe_promedio REAL,
    treynor_mediana REAL,
    beta_promedio REAL,
    perf_1y_promedio REAL,
    perf_6m_promedio REAL,
    score_promedio REAL,
    pct_positivos_1y REAL,
    tickers TEXT,
    PRIMARY KEY (run_pk, category)
);
`

// Migraciones de history.db.
var Migraciones = []string{SchemaV1}

type colMap struct{ csv, sql string }

var (
	rankingCols = []colMap{
		{"Nombre", "nombre"}, {"Category", "category"}, {"Proveedor", "proveedor"}, {"Precio", "precio"}, {"AUM", "aum"},
		{"Expense Ratio", "expense_ratio"}, {"Performance 5D (%)", "perf_5d"}, {"Performance 1M (%)", "perf_1m"},
		{"Performance 6M (%)", "perf_6m"}, {"Performance YTD (%)", "perf_ytd"}, {"Performance 1Y (%)", "perf_1y"},
		{"Sharpe Ratio", "sharpe"}, {"Treynor Ratio", "treynor"}, {"Beta", "beta"}, {"s_sharpe", "s_sharpe"},
		{"s_treynor", "s_treynor"}, {"s_1y", "s_1y"}, {"s_6m", "s_6m"}, {"s_consistencia", "s_consistencia"},
		{"s_tendencia", "s_tendencia"}, {"s_baja_dispersion", "s_baja_dispersion"}, {"s_bajo_beta", "s_bajo_beta"},
		{"s_cerca_maximo", "s_cerca_maximo"}, {"Score", "score"}, {"Score_Simple", "score_simple"},
	}
	momentumCols = []colMap{{"Nombre", "nombre"}, {"Category", "category"}, {"M_norm", "m_norm"}, {"Q_norm", "q_norm"},
		{"Consist_norm", "consist_norm"}, {"Score_comp", "score_comp"}}
	excluidosCols  = []colMap{{"Nombre", "nombre"}, {"Category", "category"}, {"Ventana_Atipica", "ventana"}, {"Z_Score", "z_score"}}
	categoriasCols = []colMap{{"Category", "category"}, {"N", "n"}, {"Sharpe_Promedio", "sharpe_promedio"},
		{"Treynor_Mediana", "treynor_mediana"}, {"Beta_Promedio", "beta_promedio"}, {"Perf_1Y_Promedio", "perf_1y_promedio"},
		{"Perf_6M_Promedio", "perf_6m_promedio"}, {"Score_Promedio", "score_promedio"}, {"Pct_Positivos_1Y", "pct_positivos_1y"},
		{"Tickers", "tickers"}}
)

var sufijoRe = regexp.MustCompile(`_(\d{8}(?:_\d+)?)\.csv$`)

func dbPath(s *core.Settings) string { return filepath.Join(ModuleDir(s), core.HistDBName) }

// CorridaEnDisco son las rutas de los CSV de una corrida ("" si ese archivo no existe).
type CorridaEnDisco struct {
	RunKey, Fecha                                  string
	Secuencia                                      int
	Ranking, Perf, Excluidos, Momentum, Categorias string
}

func oVacio(p string) string {
	if fileOK(p) {
		return p
	}
	return ""
}

// CorridasArchivadas lista las corridas de hist/ que tienen etf_ranking_completo_<sufijo>.csv, en orden.
func CorridasArchivadas(s *core.Settings) []CorridaEnDisco {
	hist := HistDir(s)
	m, _ := filepath.Glob(filepath.Join(hist, "etf_ranking_completo_*.csv"))
	var out []CorridaEnDisco
	for _, r := range m {
		sm := sufijoRe.FindStringSubmatch(filepath.Base(r))
		if sm == nil {
			continue
		}
		suf := sm[1]
		f, seq := FechaYSecuencia(filepath.Base(r))
		out = append(out, CorridaEnDisco{RunKey: suf, Fecha: f, Secuencia: seq, Ranking: r,
			Perf:       oVacio(filepath.Join(hist, "etf_perf_data_"+suf+".csv")),
			Excluidos:  oVacio(filepath.Join(hist, "etf_excluidos_outliers_"+suf+".csv")),
			Momentum:   oVacio(filepath.Join(hist, "etf_momentum_ranking_completo_"+suf+".csv")),
			Categorias: oVacio(filepath.Join(hist, "etf_categorias_ranking_"+suf+".csv"))})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Fecha != out[j].Fecha {
			return out[i].Fecha < out[j].Fecha
		}
		return out[i].Secuencia < out[j].Secuencia
	})
	return out
}

// CorridaVigente es la corrida de current/, con la clave que tendrá al archivarse.
func CorridaVigente(s *core.Settings) *CorridaEnDisco {
	cur := CurrentDir(s)
	ranking := filepath.Join(cur, RankingFile)
	perf := CSVActual(cur)
	if !fileOK(ranking) || perf == "" {
		return nil
	}
	f := FechaDe(filepath.Base(perf))
	if f == nil {
		return nil
	}
	tag := f.Format("20060102")
	n := 0
	for _, c := range CorridasArchivadas(s) {
		if c.Fecha == tag {
			n++
		}
	}
	seq := n + 1
	key := tag
	if seq != 1 {
		key = fmt.Sprintf("%s_%d", tag, seq)
	}
	return &CorridaEnDisco{RunKey: key, Fecha: tag, Secuencia: seq, Ranking: ranking, Perf: perf,
		Excluidos: oVacio(filepath.Join(cur, ExcluidosFile)), Momentum: oVacio(filepath.Join(cur, MomentumRanking)),
		Categorias: oVacio(filepath.Join(cur, CategoriasFile))}
}

type leida struct {
	c                               CorridaEnDisco
	runAt                           time.Time
	ranking                         *quant.Frame
	universoBruto                   *int64
	excluidos, momentum, categorias *quant.Frame
}

func leerCorrida(s *core.Settings, c CorridaEnDisco) (*leida, error) {
	read := func(p string) (*quant.Frame, error) {
		if p == "" {
			return nil, nil
		}
		return quant.ReadCSVFile(p)
	}
	l := &leida{c: c}
	if c.Perf != "" {
		perf, err := quant.ReadCSVFile(c.Perf)
		if err != nil {
			return nil, err
		}
		n := int64(0)
		if tc := perf.Col("TipoInstrumento"); tc != nil {
			for i := 0; i < perf.N; i++ {
				if !tc.IsNull(i) && tc.Cell(i) == "ETF" {
					n++
				}
			}
		}
		l.universoBruto = &n
	}
	st, err := os.Stat(c.Ranking)
	if err != nil {
		return nil, err
	}
	l.runAt = st.ModTime().In(s.Loc)
	if l.ranking, err = quant.ReadCSVFile(c.Ranking); err != nil {
		return nil, err
	}
	if l.excluidos, err = read(c.Excluidos); err != nil {
		return nil, err
	}
	if l.momentum, err = read(c.Momentum); err != nil {
		return nil, err
	}
	if l.categorias, err = read(c.Categorias); err != nil {
		return nil, err
	}
	return l, nil
}

func val(f *quant.Frame, col string, i int) any {
	c := f.Col(col)
	if c == nil {
		return nil
	}
	return c.Value(i)
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func insertRows(tx execer, table string, cols []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "), strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", "))
	for _, r := range rows {
		if _, err := tx.Exec(q, r...); err != nil {
			return err
		}
	}
	return nil
}

func sqlCols(m []colMap) []string {
	out := make([]string, len(m))
	for i, c := range m {
		out[i] = c.sql
	}
	return out
}

func guardarLeida(tx execer, d *leida) error {
	c, rk := d.c, d.ranking
	if _, err := tx.Exec("DELETE FROM runs WHERE run_key = ?", c.RunKey); err != nil {
		return err
	}
	var top1T, top1S, sh, be, nOut any
	if rk.N > 0 {
		top1T = rk.Col("Ticker").Cell(0)
		top1S = val(rk, "Score", 0)
	}
	if rk.Has("Sharpe Ratio") {
		sh = nanNil(quant.Mean(rk.Col("Sharpe Ratio").Floats()))
	}
	if rk.Has("Beta") {
		be = nanNil(quant.Mean(rk.Col("Beta").Floats()))
	}
	if d.excluidos != nil {
		nOut = d.excluidos.N
	}
	var bruto any
	if d.universoBruto != nil {
		bruto = *d.universoBruto
	}
	res, err := tx.Exec(`INSERT INTO runs (run_key, fecha, secuencia, run_at, universo_bruto, universo_depurado, n_outliers,
		sharpe_promedio, beta_promedio, top1_ticker, top1_score) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.RunKey, c.Fecha[:4]+"-"+c.Fecha[4:6]+"-"+c.Fecha[6:], c.Secuencia, core.FormatISO(d.runAt), bruto, rk.N, nOut, sh, be, top1T, top1S)
	if err != nil {
		return err
	}
	pk, err := res.LastInsertId()
	if err != nil {
		return err
	}
	var rows [][]any
	for i := 0; i < rk.N; i++ {
		r := []any{pk, rk.Col("Ticker").Cell(i), i + 1, boolInt(i+1 <= TopN)}
		for _, m := range rankingCols {
			r = append(r, val(rk, m.csv, i))
		}
		rows = append(rows, r)
	}
	if err := insertRows(tx, "ranking", append([]string{"run_pk", "ticker", "rank", "en_top15"}, sqlCols(rankingCols)...), rows); err != nil {
		return err
	}
	if mo := d.momentum; mo != nil {
		rows = nil
		for i := 0; i < mo.N; i++ {
			r := []any{pk, mo.Col("Ticker").Cell(i), i + 1}
			for _, m := range momentumCols {
				r = append(r, val(mo, m.csv, i))
			}
			rows = append(rows, r)
		}
		if err := insertRows(tx, "momentum", append([]string{"run_pk", "ticker", "rank"}, sqlCols(momentumCols)...), rows); err != nil {
			return err
		}
	}
	if ex := d.excluidos; ex != nil && ex.N > 0 {
		rows = nil
		for i := 0; i < ex.N; i++ {
			r := []any{pk, ex.Col("Ticker").Cell(i)}
			for _, m := range excluidosCols {
				r = append(r, val(ex, m.csv, i))
			}
			rows = append(rows, r)
		}
		if err := insertRows(tx, "excluidos", append([]string{"run_pk", "ticker"}, sqlCols(excluidosCols)...), rows); err != nil {
			return err
		}
	}
	if ca := d.categorias; ca != nil && ca.N > 0 {
		rows = nil
		for i := 0; i < ca.N; i++ {
			r := []any{pk}
			for _, m := range categoriasCols {
				r = append(r, val(ca, m.csv, i))
			}
			rows = append(rows, r)
		}
		if err := insertRows(tx, "categorias", append([]string{"run_pk"}, sqlCols(categoriasCols)...), rows); err != nil {
			return err
		}
	}
	return nil
}

func nanNil(f float64) any {
	if f != f {
		return nil
	}
	return f
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// GuardarVigente guarda (o reemplaza) la corrida de current/. Lee los CSV bajo el lock de datos y escribe la
// base afuera de él (los GET de current no esperan). Devuelve la clave guardada ("" si no había corrida).
func GuardarVigente(s *core.Settings) (string, error) {
	unlock, e := BloqueoDatos(s, 60*time.Second)
	if e != nil {
		return "", e
	}
	var d *leida
	var err error
	if c := CorridaVigente(s); c != nil {
		d, err = leerCorrida(s, *c)
	}
	unlock()
	if err != nil {
		return "", err
	}
	if d == nil {
		return "", nil
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
	if err := guardarLeida(tx, d); err != nil {
		_ = tx.Rollback()
		return "", err
	}
	return d.c.RunKey, tx.Commit()
}

// Backfill carga las corridas de hist/ y la vigente (idempotente) y devuelve el resumen.
func Backfill(s *core.Settings, warn func(format string, args ...any)) (string, error) {
	unlock, e := BloqueoDatos(s, 60*time.Second)
	if e != nil {
		return "", e
	}
	corridas := CorridasArchivadas(s)
	if v := CorridaVigente(s); v != nil {
		corridas = append(corridas, *v)
	}
	var leidas []*leida
	var omitidas []string
	for _, c := range corridas {
		d, err := leerCorrida(s, c)
		if err != nil {
			warn("backfill: se omite %s (%v)", c.RunKey, err)
			omitidas = append(omitidas, c.RunKey)
			continue
		}
		leidas = append(leidas, d)
	}
	unlock()
	if len(corridas) == 0 {
		return "backfill: no hay corridas de ETF en current/ ni en hist/", nil
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
	ok := 0
	for _, d := range leidas {
		if _, err := tx.Exec("SAVEPOINT corrida"); err != nil {
			_ = tx.Rollback()
			return "", err
		}
		if err := guardarLeida(tx, d); err != nil {
			if _, rerr := tx.Exec("ROLLBACK TO corrida"); rerr != nil {
				_ = tx.Rollback()
				return "", rerr
			}
			warn("backfill: se omite %s (%v)", d.c.RunKey, err)
			omitidas = append(omitidas, d.c.RunKey)
		} else {
			ok++
		}
		_, _ = tx.Exec("RELEASE corrida")
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	res := fmt.Sprintf("backfill: %d corridas cargadas en history.db", ok)
	if len(omitidas) > 0 {
		res += fmt.Sprintf(", %d omitidas (%s)", len(omitidas), strings.Join(omitidas, ", "))
	}
	return res, nil
}

// --- Lectura ---

const orden = "r.fecha ASC, r.secuencia ASC"

func str(f core.Fila, c string) string {
	if p := f.Str(c); p != nil {
		return *p
	}
	return ""
}

func i64(f core.Fila, c string) int64 {
	if p := f.Int(c); p != nil {
		return *p
	}
	return 0
}

func fechaDeFila(f core.Fila, col string, s *core.Settings) (*core.Fecha, *core.APIError) {
	raw := f.Str(col)
	if raw == nil {
		return nil, nil
	}
	t, err := core.ParseISO(*raw, s.Loc)
	if err != nil {
		return nil, core.Err(http.StatusServiceUnavailable, "invalid_data", "history.db: "+err.Error())
	}
	return &core.Fecha{Time: t}, nil
}

// Corridas lista las corridas guardadas.
func Corridas(s *core.Settings) ([]EtfHistCorrida, *core.APIError) {
	rows, e := core.Filas(dbPath(s), `SELECT run_key, fecha, secuencia, run_at, universo_bruto, universo_depurado, n_outliers,
		sharpe_promedio, beta_promedio, top1_ticker, top1_score FROM runs r ORDER BY `+orden)
	if e != nil {
		return nil, e
	}
	out := make([]EtfHistCorrida, 0, len(rows))
	for _, f := range rows {
		ra, e := fechaDeFila(f, "run_at", s)
		if e != nil {
			return nil, e
		}
		out = append(out, EtfHistCorrida{RunKey: str(f, "run_key"), Fecha: str(f, "fecha"), Secuencia: i64(f, "secuencia"), RunAt: ra,
			UniversoBruto: f.Int("universo_bruto"), UniversoDepurado: i64(f, "universo_depurado"), NOutliers: f.Int("n_outliers"),
			SharpePromedio: f.Float("sharpe_promedio"), BetaPromedio: f.Float("beta_promedio"),
			Top1Ticker: f.Str("top1_ticker"), Top1Score: f.Float("top1_score")})
	}
	return out, nil
}

// HistoriaTicker: cada corrida en que el ETF estuvo en el universo depurado, en el ranking de Momentum o
// excluido por outlier.
func HistoriaTicker(s *core.Settings, ticker string) ([]EtfHistTickerPunto, *core.APIError) {
	rows, e := core.Filas(dbPath(s), `
		SELECT r.run_key, r.fecha, r.universo_depurado,
		       k.rank, k.en_top15, k.score, k.score_simple, k.precio, k.sharpe, k.treynor, k.beta,
		       k.perf_1m, k.perf_6m, k.perf_1y,
		       m.rank AS momentum_rank, m.score_comp AS momentum_score,
		       x.ventana AS excluido_ventana, x.z_score AS excluido_z,
		       COALESCE(k.nombre, m.nombre, x.nombre) AS nombre,
		       COALESCE(k.category, m.category, x.category) AS category
		FROM runs r
		LEFT JOIN ranking k ON k.run_pk = r.id AND k.ticker = :t
		LEFT JOIN momentum m ON m.run_pk = r.id AND m.ticker = :t
		LEFT JOIN excluidos x ON x.run_pk = r.id AND x.ticker = :t
		WHERE k.ticker IS NOT NULL OR m.ticker IS NOT NULL OR x.ticker IS NOT NULL
		ORDER BY `+orden, sql.Named("t", ticker))
	if e != nil {
		return nil, e
	}
	if len(rows) == 0 {
		return nil, core.Err(http.StatusNotFound, "not_found", fmt.Sprintf("'%s' no aparece en el historial de ETFs.", ticker))
	}
	out := make([]EtfHistTickerPunto, 0, len(rows))
	for _, f := range rows {
		out = append(out, EtfHistTickerPunto{RunKey: str(f, "run_key"), Fecha: str(f, "fecha"), UniversoDepurado: i64(f, "universo_depurado"),
			Nombre: f.Str("nombre"), Category: f.Str("category"), Rank: f.Int("rank"), EnTop15: i64(f, "en_top15") != 0,
			Score: f.Float("score"), ScoreSimple: f.Float("score_simple"), Precio: f.Float("precio"), Sharpe: f.Float("sharpe"),
			Treynor: f.Float("treynor"), Beta: f.Float("beta"), Perf1m: f.Float("perf_1m"), Perf6m: f.Float("perf_6m"),
			Perf1y: f.Float("perf_1y"), MomentumRank: f.Int("momentum_rank"), MomentumScore: f.Float("momentum_score"),
			ExcluidoVentana: f.Str("excluido_ventana"), ExcluidoZ: f.Float("excluido_z")})
	}
	return out, nil
}

// HistoriaCategorias: score promedio y presencia en el Top 15 por categoría, en el tiempo.
func HistoriaCategorias(s *core.Settings) ([]EtfHistCategoria, *core.APIError) {
	rows, e := core.Filas(dbPath(s), `
		SELECT r.run_key, r.fecha, c.category, c.n, c.score_promedio, c.sharpe_promedio, c.perf_1y_promedio,
		       (SELECT COUNT(*) FROM ranking k
		         WHERE k.run_pk = r.id AND k.en_top15 = 1 AND k.category = c.category) AS en_top15
		FROM categorias c JOIN runs r ON r.id = c.run_pk
		ORDER BY `+orden+`, c.score_promedio DESC`)
	if e != nil {
		return nil, e
	}
	out := make([]EtfHistCategoria, 0, len(rows))
	for _, f := range rows {
		out = append(out, EtfHistCategoria{RunKey: str(f, "run_key"), Fecha: str(f, "fecha"), Category: str(f, "category"), N: f.Int("n"),
			ScorePromedio: f.Float("score_promedio"), SharpePromedio: f.Float("sharpe_promedio"),
			Perf1yPromedio: f.Float("perf_1y_promedio"), EnTop15: i64(f, "en_top15")})
	}
	return out, nil
}

// PermanenciaTop: veces en el Top 15 y racha actual en las últimas «ultimas» corridas.
func PermanenciaTop(s *core.Settings, ultimas int) ([]EtfHistPermanencia, *core.APIError) {
	rows, e := core.Filas(dbPath(s), `
		WITH ult AS (SELECT id, run_key, fecha, secuencia FROM runs ORDER BY fecha DESC, secuencia DESC LIMIT ?)
		SELECT u.run_key, k.ticker, k.nombre, k.rank, k.score
		FROM ranking k JOIN ult u ON u.id = k.run_pk
		WHERE k.en_top15 = 1
		ORDER BY u.fecha DESC, u.secuencia DESC`, ultimas)
	if e != nil {
		return nil, e
	}
	runRows, e := core.Filas(dbPath(s), "SELECT run_key FROM runs ORDER BY fecha DESC, secuencia DESC LIMIT ?", ultimas)
	if e != nil {
		return nil, e
	}
	if len(runRows) == 0 {
		return []EtfHistPermanencia{}, nil
	}
	runs := make([]string, len(runRows))
	for i, f := range runRows {
		runs[i] = str(f, "run_key")
	}
	type acc struct {
		nombre *string
		en     map[string]core.Fila
	}
	by := map[string]*acc{}
	var order []string
	for _, f := range rows {
		t := str(f, "ticker")
		d, ok := by[t]
		if !ok {
			d = &acc{nombre: f.Str("nombre"), en: map[string]core.Fila{}}
			by[t] = d
			order = append(order, t)
		}
		d.en[str(f, "run_key")] = f
	}
	out := []EtfHistPermanencia{}
	for _, t := range order {
		d := by[t]
		var racha int64
		for _, k := range runs {
			if _, ok := d.en[k]; !ok {
				break
			}
			racha++
		}
		h := EtfHistPermanencia{Ticker: t, Nombre: d.nombre, VecesEnTop: int64(len(d.en)), Corridas: int64(len(runs)), RachaActual: racha}
		if u, ok := d.en[runs[0]]; ok {
			h.RankUltima, h.ScoreUltima = u.Int("rank"), u.Float("score")
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

func rankingDe(s *core.Settings, runKey string) ([]core.Posicion, *core.APIError) {
	rows, e := core.Filas(dbPath(s), `SELECT k.ticker, k.nombre, k.rank, k.score FROM ranking k JOIN runs r ON r.id = k.run_pk WHERE r.run_key = ?`, runKey)
	if e != nil {
		return nil, e
	}
	out := make([]core.Posicion, 0, len(rows))
	for _, f := range rows {
		out = append(out, core.Posicion{Ticker: str(f, "ticker"), Nombre: f.Str("nombre"), Rank: f.Int("rank"), Score: f.Float("score")})
	}
	return out, nil
}

// Diff: rotación del Top 15 entre dos corridas cualesquiera (por defecto, las dos últimas).
func Diff(s *core.Settings, desde, hasta string) (*core.HistDiff, *core.APIError) {
	cs, e := Corridas(s)
	if e != nil {
		return nil, e
	}
	claves := map[string]bool{}
	var orden []string
	for _, c := range cs {
		claves[c.RunKey] = true
		orden = append(orden, c.RunKey)
	}
	switch {
	case desde == "" && hasta == "":
		if len(orden) < 2 {
			return nil, core.Err(http.StatusNotFound, "not_found", "Hacen falta al menos dos corridas en el historial.")
		}
		desde, hasta = orden[len(orden)-2], orden[len(orden)-1]
	case desde == "" || hasta == "":
		return nil, core.Err(http.StatusUnprocessableEntity, "validation_error", "Indica 'desde' y 'hasta', o ninguno de los dos.")
	}
	for _, k := range []string{desde, hasta} {
		if !claves[k] {
			return nil, core.Err(http.StatusNotFound, "not_found", fmt.Sprintf("No existe la corrida '%s' en el historial.", k))
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
	top := int64(TopN)
	return &core.HistDiff{Desde: desde, Hasta: hasta, TopN: &top, Filas: core.DiffCorridas(a, b, &top)}, nil
}
