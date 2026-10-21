// Package us es el módulo Acciones EEUU (/api/us): screener batch (S&P 500 ∪ Nasdaq-100 con market cap
// ≥ 200B), score de salud 0-100, portafolio (clusters de correlación + Black-Litterman) y evaluación en
// vivo de un ticker. Los nombres de campo son los del JSON que escribe el batch (snake_case).
package us

import "fogradar/internal/core"

// TickerPattern es el formato válido de un ticker en rutas y cuerpos.
const TickerPattern = `^[A-Za-z0-9.\-^=]{1,15}$`

// Macd del último día.
type Macd struct {
	Macd      float64 `json:"macd"`
	Signal    float64 `json:"signal"`
	Histogram float64 `json:"histogram"`
}

// Technical son los indicadores técnicos.
type Technical struct {
	Sma50         *float64       `json:"sma_50" nullable:"true"`
	Sma200        *float64       `json:"sma_200" nullable:"true"`
	Rsi14         *float64       `json:"rsi_14" nullable:"true"`
	Macd          core.Opt[Macd] `json:"macd"`
	Volume        *float64       `json:"volume" nullable:"true"`
	AverageVolume *float64       `json:"average_volume" nullable:"true"`
}

// Valuation son los múltiplos de valoración.
type Valuation struct {
	PerTrailing *float64 `json:"per_trailing" nullable:"true"`
	PerForward  *float64 `json:"per_forward" nullable:"true"`
	PegRatio    *float64 `json:"peg_ratio" nullable:"true"`
	EvToEbitda  *float64 `json:"ev_to_ebitda" nullable:"true"`
	PriceToBook *float64 `json:"price_to_book" nullable:"true"`
	EpsTrailing *float64 `json:"eps_trailing" nullable:"true"`
	EpsForward  *float64 `json:"eps_forward" nullable:"true"`
}

// Profitability es la rentabilidad.
type Profitability struct {
	Roe       *float64 `json:"roe" nullable:"true"`
	Roa       *float64 `json:"roa" nullable:"true"`
	NetMargin *float64 `json:"net_margin" nullable:"true"`
}

// FinancialHealth es la salud financiera.
type FinancialHealth struct {
	DebtToEquity *float64 `json:"debt_to_equity" nullable:"true"`
	CurrentRatio *float64 `json:"current_ratio" nullable:"true"`
	FreeCashFlow *float64 `json:"free_cash_flow" nullable:"true"`
}

// ShareholderReturn es el retorno al accionista.
type ShareholderReturn struct {
	DividendYield *float64 `json:"dividend_yield" nullable:"true" doc:"Fracción (0.012 = 1,2 %)."`
}

// ScoreCategories son los puntajes por categoría.
type ScoreCategories struct {
	Valuation         *float64 `json:"valuation" nullable:"true"`
	Profitability     *float64 `json:"profitability" nullable:"true"`
	FinancialHealth   *float64 `json:"financial_health" nullable:"true"`
	ShareholderReturn *float64 `json:"shareholder_return" nullable:"true"`
	Technical         *float64 `json:"technical" nullable:"true"`
}

// Score es el score de salud 0-100.
type Score struct {
	Overall    *float64        `json:"overall" nullable:"true"`
	Label      string          `json:"label"`
	Categories ScoreCategories `json:"categories"`
}

// CandidateRow es un candidato evaluado.
type CandidateRow struct {
	Ticker            string            `json:"ticker"`
	MarketCap         *float64          `json:"market_cap" nullable:"true"`
	Price             float64           `json:"price"`
	Momentum121       float64           `json:"momentum_12_1"`
	Return1d          *float64          `json:"return_1d" nullable:"true"`
	Return5d          *float64          `json:"return_5d" nullable:"true"`
	Return3m          *float64          `json:"return_3m" nullable:"true"`
	Return6m          *float64          `json:"return_6m" nullable:"true"`
	Return1y          *float64          `json:"return_1y" nullable:"true"`
	ReturnYtd         *float64          `json:"return_ytd" nullable:"true"`
	Technical         Technical         `json:"technical"`
	Name              *string           `json:"name" nullable:"true"`
	Sector            *string           `json:"sector" nullable:"true"`
	Valuation         Valuation         `json:"valuation"`
	Profitability     Profitability     `json:"profitability"`
	FinancialHealth   FinancialHealth   `json:"financial_health"`
	ShareholderReturn ShareholderReturn `json:"shareholder_return"`
	Score             Score             `json:"score"`
}

// RunMetadata de una corrida.
type RunMetadata struct {
	RunID           string     `json:"run_id"`
	Status          string     `json:"status" enum:"done,error"`
	Error           *string    `json:"error" nullable:"true"`
	StartedAt       core.Fecha `json:"started_at"`
	FinishedAt      core.Fecha `json:"finished_at"`
	MinMarketCap    float64    `json:"min_market_cap"`
	PauseSeconds    float64    `json:"pause_seconds"`
	UniverseTotal   int64      `json:"universe_total"`
	CandidatesTotal int64      `json:"candidates_total"`
	TopN            int64      `json:"top_n"`
}

// RunResponse es result.json / candidates.json.
type RunResponse struct {
	RunMetadata
	Rows []CandidateRow `json:"rows"`
}

// PriceMatrix es prices.json: fechas × tickers.
type PriceMatrix struct {
	Dates  []string              `json:"dates"`
	Prices map[string][]*float64 `json:"prices"`
}

// RunsList son las corridas disponibles.
type RunsList struct {
	Runs []string `json:"runs"`
}

// --- Historial ---

// UsHistCorrida es una corrida guardada.
type UsHistCorrida struct {
	RunID           string      `json:"run_id"`
	StartedAt       core.Fecha  `json:"started_at"`
	FinishedAt      *core.Fecha `json:"finished_at" nullable:"true"`
	UniverseTotal   *int64      `json:"universe_total" nullable:"true"`
	CandidatesTotal *int64      `json:"candidates_total" nullable:"true"`
	EvaluatedTotal  *int64      `json:"evaluated_total" nullable:"true"`
	TopN            int64       `json:"top_n"`
	Top1Ticker      *string     `json:"top1_ticker" nullable:"true"`
	Top1Score       *float64    `json:"top1_score" nullable:"true"`
}

// UsHistPilares son los pilares del score en una corrida.
type UsHistPilares struct {
	Valuation         *float64 `json:"valuation" nullable:"true"`
	Profitability     *float64 `json:"profitability" nullable:"true"`
	FinancialHealth   *float64 `json:"financial_health" nullable:"true"`
	ShareholderReturn *float64 `json:"shareholder_return" nullable:"true"`
	Technical         *float64 `json:"technical" nullable:"true"`
}

// UsHistTickerPunto es un ticker en una corrida.
type UsHistTickerPunto struct {
	RunID          string        `json:"run_id"`
	StartedAt      core.Fecha    `json:"started_at"`
	EvaluatedTotal *int64        `json:"evaluated_total" nullable:"true"`
	TopN           int64         `json:"top_n"`
	Rank           int64         `json:"rank" doc:"Posición por momentum 12-1 entre los candidatos evaluados."`
	InTop          bool          `json:"in_top"`
	Name           *string       `json:"name" nullable:"true"`
	Price          *float64      `json:"price" nullable:"true"`
	Momentum121    *float64      `json:"momentum_12_1" nullable:"true"`
	Score          *float64      `json:"score" nullable:"true"`
	ScoreLabel     *string       `json:"score_label" nullable:"true"`
	Pilares        UsHistPilares `json:"pilares"`
}

// UsHistPermanencia es la permanencia de un ticker en el top.
type UsHistPermanencia struct {
	Ticker      string   `json:"ticker"`
	Nombre      *string  `json:"nombre" nullable:"true"`
	VecesEnTop  int64    `json:"veces_en_top"`
	Corridas    int64    `json:"corridas"`
	RachaActual int64    `json:"racha_actual"`
	RankUltima  *int64   `json:"rank_ultima" nullable:"true"`
	ScoreUltima *float64 `json:"score_ultima" nullable:"true"`
}

// --- Portafolio ---

// PortfolioRequest es el cuerpo de /portfolio/optimize.
type PortfolioRequest struct {
	Tickers      []string           `json:"tickers" maxItems:"150" doc:"Universo ya filtrado (etapa 1)."`
	NAssets      int                `json:"n_assets,omitempty" default:"10" minimum:"2" maximum:"20" doc:"Activos de la cartera final (clusters)."`
	MaxWeight    float64            `json:"max_weight,omitempty" default:"0.25" minimum:"0.05" maximum:"1" doc:"Peso máximo por activo."`
	RiskFreeRate *float64           `json:"risk_free_rate,omitempty" nullable:"true" minimum:"-0.05" maximum:"0.5" doc:"Tasa libre de riesgo anual; null = valor por defecto."`
	Views        map[string]float64 `json:"views,omitempty" nullable:"true" doc:"Retorno anual esperado por ticker (fracción) que reemplaza la view automática."`
}

// PortfolioMetrics del portafolio resultante.
type PortfolioMetrics struct {
	ExpectedReturn float64 `json:"expected_return"`
	Volatility     float64 `json:"volatility"`
	Sharpe         float64 `json:"sharpe"`
}

// PortfolioPick es un activo elegido.
type PortfolioPick struct {
	Ticker  string   `json:"ticker"`
	Cluster int      `json:"cluster" doc:"Cluster de correlación (1..n) en orden de score."`
	Score   *float64 `json:"score" nullable:"true"`
}

// PortfolioResult es la respuesta de /portfolio/optimize.
type PortfolioResult struct {
	RunID            string             `json:"run_id"`
	Universe         []string           `json:"universe"`
	Selection        []PortfolioPick    `json:"selection"`
	Weights          map[string]float64 `json:"weights"`
	MaxWeight        float64            `json:"max_weight"`
	Metrics          PortfolioMetrics   `json:"metrics"`
	PriorReturns     map[string]float64 `json:"prior_returns"`
	Views            map[string]float64 `json:"views"`
	PosteriorReturns map[string]float64 `json:"posterior_returns"`
}

// PortfolioLimits son los límites y valores por defecto del modelo.
type PortfolioLimits struct {
	MinTickers          int     `json:"min_tickers"`
	MinAssets           int     `json:"min_assets"`
	MaxAssets           int     `json:"max_assets"`
	DefaultAssets       int     `json:"default_assets"`
	DefaultMaxWeight    float64 `json:"default_max_weight"`
	DefaultRiskFreeRate float64 `json:"default_risk_free_rate"`
}
