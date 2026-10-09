// Package tend es el módulo Tendencia Asia + EEUU (/api/tendencia): descubrimiento del universo por capitalización,
// precios y FX en USD, selección de portafolio (Markowitz con tope y L2), proyección a 6 meses (GBM), momentum mensual y
// semanal, informe PDF e historial. Los nombres de campo son los de dashboard_data.json y history.db del original.
package tend

import "fogradar/internal/core"

type opt = *float64

// TdMetricas son retorno, volatilidad y Sharpe anuales.
type TdMetricas struct {
	RetornoEsperadoAnual float64 `json:"retorno_esperado_anual"`
	VolatilidadAnual     float64 `json:"volatilidad_anual"`
	SharpeRatio          float64 `json:"sharpe_ratio"`
}

// TdPerformance es el desempeño del universo y del portafolio final.
type TdPerformance struct {
	UniversoCompleto     TdMetricas `json:"universo_completo"`
	Portafolio15Finales  TdMetricas `json:"portafolio_15_finales"`
	TasaLibreRiesgoAnual float64    `json:"tasa_libre_riesgo_anual"`
	TickersSeleccionados []string   `json:"tickers_seleccionados"`
}

// TdPerformanceReducida es TdMetricas con n_acciones y tickers.
type TdPerformanceReducida struct {
	TdMetricas
	NAcciones            *int64   `json:"n_acciones" nullable:"true"`
	TickersSeleccionados []string `json:"tickers_seleccionados"`
}

// TdHolding es una posición del portafolio.
type TdHolding struct {
	Ticker                  string  `json:"ticker"`
	PesoFinal               float64 `json:"peso_final"`
	Nombre                  *string `json:"nombre" nullable:"true"`
	Pais                    *string `json:"pais" nullable:"true"`
	Sector                  *string `json:"sector" nullable:"true"`
	PrecioActualUsd         opt     `json:"precio_actual_usd" nullable:"true"`
	SenalTecnica            *string `json:"senal_tecnica" nullable:"true"`
	Ma10s                   opt     `json:"ma_10s" nullable:"true"`
	Ma40s                   opt     `json:"ma_40s" nullable:"true"`
	SenalTecnica3m          *string `json:"senal_tecnica_3m" nullable:"true"`
	Ma4s                    opt     `json:"ma_4s" nullable:"true"`
	Ma13s                   opt     `json:"ma_13s" nullable:"true"`
	Momentum26sPct          opt     `json:"momentum_26s_pct" nullable:"true"`
	RetornoEsperadoAnualPct opt     `json:"retorno_esperado_anual_pct" nullable:"true"`
	VolatilidadAnualPct     opt     `json:"volatilidad_anual_pct" nullable:"true"`
	PrecioProyectado6mUsd   opt     `json:"precio_proyectado_6m_usd" nullable:"true"`
	PrecioMin90pct6mUsd     opt     `json:"precio_min_90pct_6m_usd" nullable:"true"`
	PrecioMax90pct6mUsd     opt     `json:"precio_max_90pct_6m_usd" nullable:"true"`
	RetornoEsperado6mPct    opt     `json:"retorno_esperado_6m_pct" nullable:"true"`
}

// TdSector resume un sector.
type TdSector struct {
	Sector            string  `json:"sector"`
	NAcciones         int64   `json:"n_acciones"`
	PesoTotal         float64 `json:"peso_total"`
	Retorno6mProm     opt     `json:"retorno_6m_prom" nullable:"true"`
	SenalPredominante *string `json:"senal_predominante" nullable:"true"`
	Tickers           string  `json:"tickers" doc:"Tickers del sector separados por coma (formato del original)."`
}

// TdMomentumMensual es una fila del ranking de momentum mensual.
type TdMomentumMensual struct {
	Rank                 int64   `json:"rank"`
	Ticker               string  `json:"ticker"`
	Nombre               *string `json:"nombre" nullable:"true"`
	Pais                 *string `json:"pais" nullable:"true"`
	Sector               *string `json:"sector" nullable:"true"`
	PrecioActualUsd      opt     `json:"precio_actual_usd" nullable:"true"`
	Momentum1mPct        opt     `json:"momentum_1m_pct" nullable:"true"`
	Momentum3mPct        opt     `json:"momentum_3m_pct" nullable:"true"`
	Momentum6mPct        opt     `json:"momentum_6m_pct" nullable:"true"`
	Momentum12mPct       opt     `json:"momentum_12m_pct" nullable:"true"`
	PctMesesPositivos12m opt     `json:"pct_meses_positivos_12m" nullable:"true"`
	MomentumScorePct     opt     `json:"momentum_score_pct" nullable:"true"`
}

// TdMomentumSemanal es una fila del ranking de momentum semanal.
type TdMomentumSemanal struct {
	Rank                  int64   `json:"rank"`
	Ticker                string  `json:"ticker"`
	Nombre                *string `json:"nombre" nullable:"true"`
	Pais                  *string `json:"pais" nullable:"true"`
	Sector                *string `json:"sector" nullable:"true"`
	PrecioActualUsd       opt     `json:"precio_actual_usd" nullable:"true"`
	Momentum1sPct         opt     `json:"momentum_1s_pct" nullable:"true"`
	Momentum2sPct         opt     `json:"momentum_2s_pct" nullable:"true"`
	Momentum3sPct         opt     `json:"momentum_3s_pct" nullable:"true"`
	Momentum4sPct         opt     `json:"momentum_4s_pct" nullable:"true"`
	PctSemanasPositivas4s opt     `json:"pct_semanas_positivas_4s" nullable:"true"`
	MomentumScorePct      opt     `json:"momentum_score_pct" nullable:"true"`
}

// TdPortafolioReducido es un portafolio de N acciones re-optimizado.
type TdPortafolioReducido struct {
	N           int64                 `json:"n"`
	Performance TdPerformanceReducida `json:"performance"`
	Holdings    []TdHolding           `json:"holdings"`
}

// TdAnalisis es un análisis de la corrida (Asia + EEUU o solo EEUU).
type TdAnalisis struct {
	Key              string                 `json:"key" doc:"\"\" = Asia + EEUU (principal), \"_us\" = solo EEUU."`
	Label            string                 `json:"label"`
	UniverseSize     int64                  `json:"universe_size"`
	NSelected        int64                  `json:"n_selected"`
	Performance      TdPerformance          `json:"performance"`
	Holdings         []TdHolding            `json:"holdings"`
	Sectors          []TdSector             `json:"sectors"`
	Momentum         []TdMomentumMensual    `json:"momentum"`
	MomentumSemanal  []TdMomentumSemanal    `json:"momentum_semanal"`
	PortfolioOptions []TdPortafolioReducido `json:"portfolio_options"`
}

// TdArchivo es un archivo descargable de una corrida.
type TdArchivo struct {
	Nombre string `json:"nombre"`
	Tipo   string `json:"tipo"`
	Bytes  int64  `json:"bytes"`
}

// TdParametros son los parámetros del modelo (para los textos de metodología).
type TdParametros struct {
	TopN              int64   `json:"top_n"`
	TopNUs            int64   `json:"top_n_us"`
	MaxPesoPorActivo  float64 `json:"max_peso_por_activo"`
	L2Gamma           float64 `json:"l2_gamma"`
	SemanasMinimas    int64   `json:"semanas_minimas"`
	HorizonteSemanas  int64   `json:"horizonte_semanas"`
	MaCorta           int64   `json:"ma_corta"`
	MaLarga           int64   `json:"ma_larga"`
	MaCorta3m         int64   `json:"ma_corta_3m"`
	MaLarga3m         int64   `json:"ma_larga_3m"`
	TopNMomentum      int64   `json:"top_n_momentum"`
	TamanosPortafolio []int64 `json:"tamanos_portafolio"`
}

// TdExcluido es un ticker del universo que quedó fuera del análisis.
type TdExcluido struct {
	Ticker string `json:"ticker"`
	Nombre string `json:"nombre"`
	Motivo string `json:"motivo"`
}

// TdDashboard es una corrida completa (mismo contenido que dashboard_data.json).
type TdDashboard struct {
	GeneratedAt       core.Fecha             `json:"generated_at"`
	RunDir            string                 `json:"run_dir"`
	UniverseSource    *string                `json:"universe_source" nullable:"true"`
	MarketRegimeNotes []string               `json:"market_regime_notes"`
	Analyses          []TdAnalisis           `json:"analyses"`
	Archivos          []TdArchivo            `json:"archivos" doc:"Archivos descargables de la corrida (PDF, CSV…), ver /runs/{run_dir}/files."`
	Parametros        core.Opt[TdParametros] `json:"parametros"`
	Excluidos         []TdExcluido           `json:"excluidos" doc:"Tickers excluidos en el último armado del dataset (TD-DATA-1 / TD-OPS-2)."`
}

// --- Historial (history.db) ---

// TdAnalisisDisponible es un análisis visto en el historial.
type TdAnalisisDisponible struct {
	AnalysisKey string `json:"analysis_key"`
	Label       string `json:"label"`
}

// TdRunPerformance es el desempeño de una corrida.
type TdRunPerformance struct {
	RunDir                     string     `json:"run_dir"`
	RunDate                    string     `json:"run_date" format:"date"`
	GeneratedAt                core.Fecha `json:"generated_at"`
	UniverseSource             *string    `json:"universe_source" nullable:"true"`
	AnalysisKey                string     `json:"analysis_key"`
	Label                      string     `json:"label"`
	UniverseSize               *int64     `json:"universe_size" nullable:"true"`
	NSelected                  *int64     `json:"n_selected" nullable:"true"`
	RetornoUniversoAnual       opt        `json:"retorno_universo_anual" nullable:"true"`
	VolatilidadUniversoAnual   opt        `json:"volatilidad_universo_anual" nullable:"true"`
	SharpeUniverso             opt        `json:"sharpe_universo" nullable:"true"`
	RetornoPortafolioAnual     opt        `json:"retorno_portafolio_anual" nullable:"true"`
	VolatilidadPortafolioAnual opt        `json:"volatilidad_portafolio_anual" nullable:"true"`
	SharpePortafolio           opt        `json:"sharpe_portafolio" nullable:"true"`
	TasaLibreRiesgoAnual       opt        `json:"tasa_libre_riesgo_anual" nullable:"true"`
}

// TdTickerNombre es un ticker seleccionado alguna vez.
type TdTickerNombre struct {
	Ticker string  `json:"ticker"`
	Nombre *string `json:"nombre" nullable:"true"`
}

// TdTickerHistoria es la trayectoria de un ticker.
type TdTickerHistoria struct {
	RunDate               string  `json:"run_date" format:"date"`
	RunDir                string  `json:"run_dir"`
	PesoFinal             opt     `json:"peso_final" nullable:"true"`
	PrecioActualUsd       opt     `json:"precio_actual_usd" nullable:"true"`
	SenalTecnica          *string `json:"senal_tecnica" nullable:"true"`
	Momentum26sPct        opt     `json:"momentum_26s_pct" nullable:"true"`
	RetornoEsperado6mPct  opt     `json:"retorno_esperado_6m_pct" nullable:"true"`
	PrecioProyectado6mUsd opt     `json:"precio_proyectado_6m_usd" nullable:"true"`
	PrecioMin90pct6mUsd   opt     `json:"precio_min_90pct_6m_usd" nullable:"true"`
	PrecioMax90pct6mUsd   opt     `json:"precio_max_90pct_6m_usd" nullable:"true"`
}

// TdUniversoHistoria es la posición de un ticker en el universo completo.
type TdUniversoHistoria struct {
	RunDate              string `json:"run_date" format:"date"`
	RunDir               string `json:"run_dir"`
	UniverseRank         int64  `json:"universe_rank"`
	UniverseSize         *int64 `json:"universe_size" nullable:"true"`
	PesoMarkowitz        opt    `json:"peso_markowitz" nullable:"true"`
	RetornoEsperadoAnual opt    `json:"retorno_esperado_anual" nullable:"true"`
	Seleccionado         bool   `json:"seleccionado"`
}

// TdSectorHistoria es el peso de un sector en el tiempo.
type TdSectorHistoria struct {
	RunDate       string  `json:"run_date" format:"date"`
	RunDir        string  `json:"run_dir"`
	Sector        *string `json:"sector" nullable:"true"`
	PesoTotal     opt     `json:"peso_total" nullable:"true"`
	NAcciones     *int64  `json:"n_acciones" nullable:"true"`
	Retorno6mProm opt     `json:"retorno_6m_prom" nullable:"true"`
}

// TdMomentumLider es un líder de momentum de una corrida.
type TdMomentumLider struct {
	RunDate          string  `json:"run_date" format:"date"`
	RunDir           string  `json:"run_dir"`
	Ticker           *string `json:"ticker" nullable:"true"`
	MomentumRank     *int64  `json:"momentum_rank" nullable:"true"`
	MomentumScorePct opt     `json:"momentum_score_pct" nullable:"true"`
}

// TdRunDetalle es una corrida reconstruida desde history.db.
type TdRunDetalle struct {
	_                 struct{}         `json:"-" additionalProperties:"true"`
	ID                int64            `json:"id"`
	RunDir            string           `json:"run_dir"`
	RunDate           string           `json:"run_date" format:"date"`
	GeneratedAt       core.Fecha       `json:"generated_at"`
	UniverseSource    *string          `json:"universe_source" nullable:"true"`
	MarketRegimeNotes []string         `json:"market_regime_notes"`
	StoredAt          *string          `json:"stored_at" nullable:"true"`
	Analyses          []map[string]any `json:"analyses"`
}
