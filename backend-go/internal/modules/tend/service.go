package tend

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"fogradar/internal/core"
)

func nz[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// normalize deja las listas ausentes (corridas antiguas) como [] en vez de null.
func (d *TdDashboard) normalize() {
	d.MarketRegimeNotes, d.Archivos, d.Excluidos = nz(d.MarketRegimeNotes), nz(d.Archivos), nz(d.Excluidos)
	d.Analyses = nz(d.Analyses)
	for i := range d.Analyses {
		a := &d.Analyses[i]
		a.Holdings, a.Sectors, a.Momentum = nz(a.Holdings), nz(a.Sectors), nz(a.Momentum)
		a.MomentumSemanal, a.PortfolioOptions = nz(a.MomentumSemanal), nz(a.PortfolioOptions)
		a.RetornoReciente = nz(a.RetornoReciente)
		a.Performance.TickersSeleccionados = nz(a.Performance.TickersSeleccionados)
		for j := range a.PortfolioOptions {
			o := &a.PortfolioOptions[j]
			o.Holdings = nz(o.Holdings)
			o.Performance.TickersSeleccionados = nz(o.Performance.TickersSeleccionados)
		}
	}
}

// Dashboard sirve una corrida completa (la última si runDir == "").
func Dashboard(s *core.Settings, runDir string) (*TdDashboard, *core.APIError) {
	var d string
	var e *core.APIError
	if runDir == "" {
		d, e = LatestRun(s)
	} else {
		d, e = RunDir(s, runDir)
	}
	if e != nil {
		return nil, e
	}
	var out TdDashboard
	if e := ReadDashboard(d, &out); e != nil {
		return nil, e
	}
	out.Archivos = Archivos(d)
	out.Parametros = core.Some(DefaultParams().Td())
	out.Excluidos = []TdExcluido{}
	if runDir == "" { // solo aplica a la última corrida: el archivo se reescribe en cada armado
		out.Excluidos = Excluidos(s)
	}
	out.normalize()
	return &out, nil
}

// --- history.db ---

func strp(f core.Fila, c string) *string { return f.Str(c) }

func sv(f core.Fila, c string) string {
	if p := f.Str(c); p != nil {
		return *p
	}
	return ""
}

func fechaGen(f core.Fila, s *core.Settings) (core.Fecha, *core.APIError) {
	t, err := core.ParseISO(sv(f, "generated_at"), s.Loc)
	if err != nil {
		return core.Fecha{}, core.Err(http.StatusServiceUnavailable, "invalid_data", "history.db: "+err.Error())
	}
	return core.Fecha{Time: t}, nil
}

// HistAnalyses lista los análisis vistos en el historial.
func HistAnalyses(s *core.Settings) ([]TdAnalisisDisponible, *core.APIError) {
	rows, e := core.Filas(HistoryDB(s), "SELECT DISTINCT analysis_key, label FROM analyses ORDER BY analysis_key")
	if e != nil {
		return nil, e
	}
	out := make([]TdAnalisisDisponible, 0, len(rows))
	for _, f := range rows {
		out = append(out, TdAnalisisDisponible{AnalysisKey: sv(f, "analysis_key"), Label: sv(f, "label")})
	}
	return out, nil
}

// Runs es la serie de performance por corrida (ascendente).
func Runs(s *core.Settings, analysis, from, to string, limit int) ([]TdRunPerformance, *core.APIError) {
	q := `SELECT r.run_dir, r.run_date, r.generated_at, r.universe_source,
		       a.analysis_key, a.label, a.universe_size, a.n_selected,
		       a.retorno_universo_anual, a.volatilidad_universo_anual, a.sharpe_universo,
		       a.retorno_portafolio_anual, a.volatilidad_portafolio_anual, a.sharpe_portafolio,
		       a.tasa_libre_riesgo_anual
		FROM runs r JOIN analyses a ON a.run_id = r.id WHERE a.analysis_key = ?`
	args := []any{analysis}
	if from != "" {
		q += " AND r.run_date >= ?"
		args = append(args, from)
	}
	if to != "" {
		q += " AND r.run_date <= ?"
		args = append(args, to)
	}
	q += " ORDER BY r.run_date ASC, r.generated_at ASC LIMIT ?"
	args = append(args, limit)
	rows, e := core.Filas(HistoryDB(s), q, args...)
	if e != nil {
		return nil, e
	}
	out := make([]TdRunPerformance, 0, len(rows))
	for _, f := range rows {
		g, e := fechaGen(f, s)
		if e != nil {
			return nil, e
		}
		out = append(out, TdRunPerformance{RunDir: sv(f, "run_dir"), RunDate: sv(f, "run_date"), GeneratedAt: g,
			UniverseSource: strp(f, "universe_source"), AnalysisKey: sv(f, "analysis_key"), Label: sv(f, "label"),
			UniverseSize: f.Int("universe_size"), NSelected: f.Int("n_selected"),
			RetornoUniversoAnual: f.Float("retorno_universo_anual"), VolatilidadUniversoAnual: f.Float("volatilidad_universo_anual"),
			SharpeUniverso: f.Float("sharpe_universo"), RetornoPortafolioAnual: f.Float("retorno_portafolio_anual"),
			VolatilidadPortafolioAnual: f.Float("volatilidad_portafolio_anual"), SharpePortafolio: f.Float("sharpe_portafolio"),
			TasaLibreRiesgoAnual: f.Float("tasa_libre_riesgo_anual")})
	}
	return out, nil
}

func allRows(db *sql.DB, q string, args ...any) ([]map[string]any, error) {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				m[c] = string(b)
			} else {
				m[c] = vals[i]
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// RunDetalle reconstruye una corrida desde history.db.
func RunDetalle(s *core.Settings, runDir string) (*TdRunDetalle, *core.APIError) {
	notFound := core.Err(http.StatusNotFound, "not_found", fmt.Sprintf("No existe la corrida '%s' en la base.", runDir))
	db, err := core.OpenRead(HistoryDB(s))
	if err != nil {
		return nil, core.Err(http.StatusServiceUnavailable, "invalid_data", err.Error())
	}
	if db == nil {
		return nil, notFound
	}
	defer db.Close()
	runs, err := allRows(db, "SELECT * FROM runs WHERE run_dir = ?", runDir)
	if err != nil {
		return nil, core.Err(http.StatusServiceUnavailable, "invalid_data", "history.db no se pudo leer: "+err.Error())
	}
	if len(runs) == 0 {
		return nil, notFound
	}
	run := runs[0]
	analyses, err := allRows(db, "SELECT * FROM analyses WHERE run_id = ?", run["id"])
	if err != nil {
		return nil, core.Err(http.StatusServiceUnavailable, "invalid_data", err.Error())
	}
	detalle := make([]map[string]any, 0, len(analyses))
	for _, a := range analyses {
		for key, q := range map[string]string{
			"holdings": "SELECT * FROM holdings WHERE analysis_id = ? ORDER BY peso_final DESC",
			"sectors":  "SELECT * FROM sectors WHERE analysis_id = ? ORDER BY peso_total DESC",
			"momentum": "SELECT * FROM momentum WHERE analysis_id = ? ORDER BY momentum_rank ASC",
		} {
			rows, err := allRows(db, q, a["id"])
			if err != nil {
				return nil, core.Err(http.StatusServiceUnavailable, "invalid_data", err.Error())
			}
			a[key] = rows
		}
		detalle = append(detalle, a)
	}
	out := &TdRunDetalle{RunDir: runDir, Analyses: detalle, MarketRegimeNotes: []string{}}
	if v, ok := run["id"].(int64); ok {
		out.ID = v
	}
	out.RunDate, _ = run["run_date"].(string)
	if g, ok := run["generated_at"].(string); ok {
		t, err := core.ParseISO(g, s.Loc)
		if err != nil {
			return nil, core.Err(http.StatusServiceUnavailable, "invalid_data", err.Error())
		}
		out.GeneratedAt = core.Fecha{Time: t}
	}
	if v, ok := run["universe_source"].(string); ok {
		out.UniverseSource = &v
	}
	if v, ok := run["stored_at"].(string); ok {
		out.StoredAt = &v
	}
	if v, ok := run["market_regime_notes"].(string); ok && v != "" {
		_ = json.Unmarshal([]byte(v), &out.MarketRegimeNotes)
	}
	out.MarketRegimeNotes = nz(out.MarketRegimeNotes)
	return out, nil
}

// Tickers: tickers seleccionados alguna vez en un análisis.
func Tickers(s *core.Settings, analysis string) ([]TdTickerNombre, *core.APIError) {
	rows, e := core.Filas(HistoryDB(s), `SELECT DISTINCT h.ticker, h.nombre FROM holdings h JOIN analyses a ON a.id = h.analysis_id
		WHERE a.analysis_key = ? ORDER BY h.ticker`, analysis)
	if e != nil {
		return nil, e
	}
	out := make([]TdTickerNombre, 0, len(rows))
	for _, f := range rows {
		out = append(out, TdTickerNombre{Ticker: sv(f, "ticker"), Nombre: strp(f, "nombre")})
	}
	return out, nil
}

func nombreAnalisis(a string) string {
	if a == "" {
		return "principal"
	}
	return a
}

// TickerHistory es la trayectoria de un ticker en las corridas.
func TickerHistory(s *core.Settings, ticker, analysis string) ([]TdTickerHistoria, *core.APIError) {
	rows, e := core.Filas(HistoryDB(s), `SELECT r.run_date, r.run_dir, h.peso_final, h.precio_actual_usd, h.senal_tecnica,
		       h.momentum_26s_pct, h.retorno_esperado_6m_pct, h.precio_proyectado_6m_usd,
		       h.precio_min_90pct_6m_usd, h.precio_max_90pct_6m_usd
		FROM holdings h JOIN analyses a ON a.id = h.analysis_id JOIN runs r ON r.id = a.run_id
		WHERE a.analysis_key = ? AND h.ticker = ? ORDER BY r.run_date ASC`, analysis, ticker)
	if e != nil {
		return nil, e
	}
	if len(rows) == 0 {
		return nil, core.Err(http.StatusNotFound, "not_found", fmt.Sprintf("'%s' nunca apareció seleccionado en el análisis '%s'.", ticker, nombreAnalisis(analysis)))
	}
	out := make([]TdTickerHistoria, 0, len(rows))
	for _, f := range rows {
		out = append(out, TdTickerHistoria{RunDate: sv(f, "run_date"), RunDir: sv(f, "run_dir"), PesoFinal: f.Float("peso_final"),
			PrecioActualUsd: f.Float("precio_actual_usd"), SenalTecnica: strp(f, "senal_tecnica"), Momentum26sPct: f.Float("momentum_26s_pct"),
			RetornoEsperado6mPct: f.Float("retorno_esperado_6m_pct"), PrecioProyectado6mUsd: f.Float("precio_proyectado_6m_usd"),
			PrecioMin90pct6mUsd: f.Float("precio_min_90pct_6m_usd"), PrecioMax90pct6mUsd: f.Float("precio_max_90pct_6m_usd")})
	}
	return out, nil
}

// SectorHistory es el peso por sector en el tiempo.
func SectorHistory(s *core.Settings, analysis string) ([]TdSectorHistoria, *core.APIError) {
	rows, e := core.Filas(HistoryDB(s), `SELECT r.run_date, r.run_dir, s.sector, s.peso_total, s.n_acciones, s.retorno_6m_prom
		FROM sectors s JOIN analyses a ON a.id = s.analysis_id JOIN runs r ON r.id = a.run_id
		WHERE a.analysis_key = ? ORDER BY r.run_date ASC`, analysis)
	if e != nil {
		return nil, e
	}
	out := make([]TdSectorHistoria, 0, len(rows))
	for _, f := range rows {
		out = append(out, TdSectorHistoria{RunDate: sv(f, "run_date"), RunDir: sv(f, "run_dir"), Sector: strp(f, "sector"),
			PesoTotal: f.Float("peso_total"), NAcciones: f.Int("n_acciones"), Retorno6mProm: f.Float("retorno_6m_prom")})
	}
	return out, nil
}

// MomentumLeaders son los líderes de momentum por corrida.
func MomentumLeaders(s *core.Settings, analysis string, top int) ([]TdMomentumLider, *core.APIError) {
	rows, e := core.Filas(HistoryDB(s), `SELECT r.run_date, r.run_dir, m.ticker, m.momentum_rank, m.momentum_score_pct
		FROM momentum m JOIN analyses a ON a.id = m.analysis_id JOIN runs r ON r.id = a.run_id
		WHERE a.analysis_key = ? AND m.momentum_rank <= ? ORDER BY r.run_date ASC, m.momentum_rank ASC`, analysis, top)
	if e != nil {
		return nil, e
	}
	out := make([]TdMomentumLider, 0, len(rows))
	for _, f := range rows {
		out = append(out, TdMomentumLider{RunDate: sv(f, "run_date"), RunDir: sv(f, "run_dir"), Ticker: strp(f, "ticker"),
			MomentumRank: f.Int("momentum_rank"), MomentumScorePct: f.Float("momentum_score_pct")})
	}
	return out, nil
}

// UniverseHistory: posición de un ticker en el universo completo (pesos de Markowitz) en cada corrida.
func UniverseHistory(s *core.Settings, ticker, analysis string) ([]TdUniversoHistoria, *core.APIError) {
	rows, e := core.Filas(HistoryDB(s), `SELECT r.run_date, r.run_dir, u.universe_rank, a.universe_size, u.peso_markowitz,
		       u.retorno_esperado_anual,
		       EXISTS (SELECT 1 FROM holdings h WHERE h.analysis_id = a.id AND h.ticker = u.ticker) AS seleccionado
		FROM universe_weights u JOIN analyses a ON a.id = u.analysis_id JOIN runs r ON r.id = a.run_id
		WHERE a.analysis_key = ? AND u.ticker = ? ORDER BY r.run_date ASC, r.run_dir ASC`, analysis, ticker)
	if e != nil {
		return nil, e
	}
	if len(rows) == 0 {
		return nil, core.Err(http.StatusNotFound, "not_found", fmt.Sprintf("'%s' no tiene pesos del universo guardados en el análisis '%s'.", ticker, nombreAnalisis(analysis)))
	}
	out := make([]TdUniversoHistoria, 0, len(rows))
	for _, f := range rows {
		rk := int64(0)
		if v := f.Int("universe_rank"); v != nil {
			rk = *v
		}
		sel := false
		if v := f.Int("seleccionado"); v != nil {
			sel = *v != 0
		}
		out = append(out, TdUniversoHistoria{RunDate: sv(f, "run_date"), RunDir: sv(f, "run_dir"), UniverseRank: rk,
			UniverseSize: f.Int("universe_size"), PesoMarkowitz: f.Float("peso_markowitz"),
			RetornoEsperadoAnual: f.Float("retorno_esperado_anual"), Seleccionado: sel})
	}
	return out, nil
}
