package etf

import "fogradar/internal/core"

// Modelos de la API del módulo ETF (/api/etf). En disco los CSV conservan los nombres de columna del original;
// la API los expone en snake_case estable. Todos los campos están siempre presentes (null si faltan).

type opt = *float64

// EtfFila es un ETF en cualquiera de las tablas (cada tabla trae solo algunos campos).
type EtfFila struct {
	Ticker             string  `json:"ticker"`
	Nombre             *string `json:"nombre" nullable:"true"`
	Categoria          *string `json:"categoria" nullable:"true"`
	Proveedor          *string `json:"proveedor" nullable:"true"`
	Precio             opt     `json:"precio" nullable:"true"`
	Volumen            opt     `json:"volumen" nullable:"true"`
	Aum                opt     `json:"aum" nullable:"true"`
	ExpenseRatio       opt     `json:"expense_ratio" nullable:"true"`
	Perf5d             opt     `json:"perf_5d" nullable:"true"`
	Perf1m             opt     `json:"perf_1m" nullable:"true"`
	Perf6m             opt     `json:"perf_6m" nullable:"true"`
	PerfYtd            opt     `json:"perf_ytd" nullable:"true"`
	Perf1y             opt     `json:"perf_1y" nullable:"true"`
	Perf5y             opt     `json:"perf_5y" nullable:"true"`
	Max52s             opt     `json:"max_52s" nullable:"true"`
	Min52s             opt     `json:"min_52s" nullable:"true"`
	Sharpe             opt     `json:"sharpe" nullable:"true"`
	Treynor            opt     `json:"treynor" nullable:"true"`
	Beta               opt     `json:"beta" nullable:"true"`
	DistMax52sPct      opt     `json:"dist_max_52s_pct" nullable:"true"`
	Rango52sPct        opt     `json:"rango_52s_pct" nullable:"true"`
	PeriodosPositivos  opt     `json:"periodos_positivos" nullable:"true"`
	DispersionPeriodos opt     `json:"dispersion_periodos" nullable:"true"`
	PasosCrecientes    opt     `json:"pasos_crecientes" nullable:"true"`
	SSharpe            opt     `json:"s_sharpe" nullable:"true"`
	STreynor           opt     `json:"s_treynor" nullable:"true"`
	S1y                opt     `json:"s_1y" nullable:"true"`
	S6m                opt     `json:"s_6m" nullable:"true"`
	SConsistencia      opt     `json:"s_consistencia" nullable:"true"`
	STendencia         opt     `json:"s_tendencia" nullable:"true"`
	SBajaDispersion    opt     `json:"s_baja_dispersion" nullable:"true"`
	SBajoBeta          opt     `json:"s_bajo_beta" nullable:"true"`
	SCercaMaximo       opt     `json:"s_cerca_maximo" nullable:"true"`
	Score              opt     `json:"score" nullable:"true"`
	Ss5d               opt     `json:"ss_5d" nullable:"true"`
	Ss1m               opt     `json:"ss_1m" nullable:"true"`
	Ss6m               opt     `json:"ss_6m" nullable:"true"`
	Ss1y               opt     `json:"ss_1y" nullable:"true"`
	SsBajoBeta         opt     `json:"ss_bajo_beta" nullable:"true"`
	ScoreSimple        opt     `json:"score_simple" nullable:"true"`
	VentanaAtipica     *string `json:"ventana_atipica" nullable:"true"`
	ZScore             opt     `json:"z_score" nullable:"true"`
	MNorm              opt     `json:"m_norm" nullable:"true"`
	QNorm              opt     `json:"q_norm" nullable:"true"`
	ConsistNorm        opt     `json:"consist_norm" nullable:"true"`
	ScoreComp          opt     `json:"score_comp" nullable:"true"`
}

// EtfTabla es una tabla de ETFs con su fecha.
type EtfTabla struct {
	Fecha *string   `json:"fecha" format:"date" nullable:"true"`
	Filas []EtfFila `json:"filas"`
}

// EtfRotacionFila es una fila de rotación.
type EtfRotacionFila struct {
	EtfFila
	Racha       *int64  `json:"racha" nullable:"true"`
	RankActual  *int64  `json:"rank_actual" nullable:"true"`
	RankPrevio  *int64  `json:"rank_previo" nullable:"true"`
	DeltaScore  opt     `json:"delta_score" nullable:"true"`
	DeltaRank   *int64  `json:"delta_rank" nullable:"true"`
	ScorePrevio opt     `json:"score_previo" nullable:"true"`
	Procedencia *string `json:"procedencia" nullable:"true"`
	Destino     *string `json:"destino" nullable:"true"`
}

// EtfRotacion es la rotación del Top 15 vs. la corrida anterior.
type EtfRotacion struct {
	Disponible  bool              `json:"disponible" doc:"False si todavía no hay una corrida archivada con la cual comparar."`
	Fecha       *string           `json:"fecha" format:"date" nullable:"true"`
	FechaPrevia *string           `json:"fecha_previa" format:"date" nullable:"true"`
	Mantienen   []EtfRotacionFila `json:"mantienen"`
	Nuevos      []EtfRotacionFila `json:"nuevos"`
	Salidas     []EtfRotacionFila `json:"salidas"`
}

// EtfDiversificacionFila es la cuenta por categoría.
type EtfDiversificacionFila struct {
	Categoria string  `json:"categoria"`
	Conteo    int64   `json:"conteo"`
	Pct       float64 `json:"pct"`
}

// EtfDiversificacion es la diversificación del Top 15.
type EtfDiversificacion struct {
	Fecha *string                  `json:"fecha" format:"date" nullable:"true"`
	Filas []EtfDiversificacionFila `json:"filas"`
}

// EtfCategoriaFila es una categoría del ranking por categoría.
type EtfCategoriaFila struct {
	Categoria      string   `json:"categoria"`
	N              int64    `json:"n"`
	SharpePromedio opt      `json:"sharpe_promedio" nullable:"true"`
	TreynorMediana opt      `json:"treynor_mediana" nullable:"true"`
	BetaPromedio   opt      `json:"beta_promedio" nullable:"true"`
	Perf1yPromedio opt      `json:"perf_1y_promedio" nullable:"true"`
	Perf6mPromedio opt      `json:"perf_6m_promedio" nullable:"true"`
	ScorePromedio  opt      `json:"score_promedio" nullable:"true"`
	PctPositivos1y opt      `json:"pct_positivos_1y" nullable:"true"`
	Tickers        []string `json:"tickers" doc:"Tickers de la categoría, ordenados por Sharpe descendente."`
}

// EtfCategorias es la respuesta de /categorias/ranking.
type EtfCategorias struct {
	Fecha         *string            `json:"fecha" format:"date" nullable:"true"`
	MinNCategoria int64              `json:"min_n_categoria"`
	Filas         []EtfCategoriaFila `json:"filas"`
}

// EtfCapFila es una fila del Top 15 Cap.
type EtfCapFila struct {
	EtfFila
	CapBucket      *string `json:"cap_bucket" nullable:"true"`
	CompT12t1Aprox opt     `json:"comp_t12t1_aprox" nullable:"true"`
	CompT7t1Aprox  opt     `json:"comp_t7t1_aprox" nullable:"true"`
	AlphaCapmAprox opt     `json:"alpha_capm_aprox" nullable:"true"`
	Comp1Norm      opt     `json:"comp1_norm" nullable:"true"`
	Comp2Norm      opt     `json:"comp2_norm" nullable:"true"`
	Comp3Norm      opt     `json:"comp3_norm" nullable:"true"`
	MomentumScore  opt     `json:"momentum_score" nullable:"true"`
	PesoPortfolio  opt     `json:"peso_portfolio" nullable:"true"`
}

// EtfCapBucket resume un segmento de capitalización.
type EtfCapBucket struct {
	CapBucket       string `json:"cap_bucket"`
	NUniverso       int64  `json:"n_universo"`
	AumTotalBucket  opt    `json:"aum_total_bucket" nullable:"true"`
	NSeleccionados  int64  `json:"n_seleccionados"`
	AumSeleccionado opt    `json:"aum_seleccionado" nullable:"true"`
	PctAumCubierto  opt    `json:"pct_aum_cubierto" nullable:"true"`
}

// EtfCap es la respuesta de /russell3000.
type EtfCap struct {
	Fecha              *string        `json:"fecha" format:"date" nullable:"true"`
	BenchmarkTicker    *string        `json:"benchmark_ticker" nullable:"true"`
	BenchmarkRetorno1y opt            `json:"benchmark_retorno_1y" nullable:"true"`
	Filas              []EtfCapFila   `json:"filas"`
	Buckets            []EtfCapBucket `json:"buckets"`
}

// EtfHistorialFila es una fila de historial_corridas.csv.
type EtfHistorialFila struct {
	Fecha            string  `json:"fecha" format:"date"`
	UniversoBruto    int64   `json:"universo_bruto"`
	UniversoDepurado int64   `json:"universo_depurado"`
	NOutliers        int64   `json:"n_outliers"`
	SharpePromedio   opt     `json:"sharpe_promedio" nullable:"true"`
	BetaPromedio     opt     `json:"beta_promedio" nullable:"true"`
	Top1Ticker       *string `json:"top1_ticker" nullable:"true"`
	Top1Score        opt     `json:"top1_score" nullable:"true"`
}

// EtfHistorial es la respuesta de /historial.
type EtfHistorial struct {
	Filas []EtfHistorialFila `json:"filas" doc:"De la corrida más antigua a la más reciente."`
}

// EtfPeso es un peso del modelo.
type EtfPeso struct {
	Clave    string  `json:"clave"`
	Etiqueta string  `json:"etiqueta"`
	Peso     float64 `json:"peso"`
}

// EtfModelo son los parámetros de las metodologías (el front no los duplica).
type EtfModelo struct {
	TopN                        int64     `json:"top_n"`
	Pesos                       []EtfPeso `json:"pesos"`
	PesosSimple                 []EtfPeso `json:"pesos_simple"`
	PesosMomentum               []EtfPeso `json:"pesos_momentum"`
	PesosMomentumM              []EtfPeso `json:"pesos_momentum_m"`
	ZThreshold                  float64   `json:"z_threshold"`
	LiqVolMin                   float64   `json:"liq_vol_min"`
	LiqAumMin                   float64   `json:"liq_aum_min"`
	CategoriasExcluidas         []string  `json:"categorias_excluidas"`
	CategoriasExcluidasMomentum []string  `json:"categorias_excluidas_momentum"`
	MinNCategoria               int64     `json:"min_n_categoria"`
	CapNPorBucket               int64     `json:"cap_n_por_bucket"`
	CapTopePesoPct              float64   `json:"cap_tope_peso_pct"`
	CapBenchmarks               []string  `json:"cap_benchmarks"`
}

// EtfResumen es la respuesta de /resumen.
type EtfResumen struct {
	Fecha                *string                    `json:"fecha" format:"date" nullable:"true"`
	ArchivoCsv           string                     `json:"archivo_csv"`
	GeneradoEn           *core.Fecha                `json:"generado_en" nullable:"true"`
	TieneCorridaAnterior bool                       `json:"tiene_corrida_anterior"`
	Resumen              core.Opt[EtfHistorialFila] `json:"resumen"`
	Modelo               EtfModelo                  `json:"modelo"`
}

// --- Historial (history.db) ---

// EtfHistCorrida es una corrida guardada.
type EtfHistCorrida struct {
	RunKey           string      `json:"run_key" doc:"<yyyymmdd> o <yyyymmdd>_<n> (mismo sufijo que hist/)."`
	Fecha            string      `json:"fecha" format:"date"`
	Secuencia        int64       `json:"secuencia"`
	RunAt            *core.Fecha `json:"run_at" nullable:"true"`
	UniversoBruto    *int64      `json:"universo_bruto" nullable:"true"`
	UniversoDepurado int64       `json:"universo_depurado"`
	NOutliers        *int64      `json:"n_outliers" nullable:"true"`
	SharpePromedio   opt         `json:"sharpe_promedio" nullable:"true"`
	BetaPromedio     opt         `json:"beta_promedio" nullable:"true"`
	Top1Ticker       *string     `json:"top1_ticker" nullable:"true"`
	Top1Score        opt         `json:"top1_score" nullable:"true"`
}

// EtfHistTickerPunto es un ETF en una corrida.
type EtfHistTickerPunto struct {
	RunKey           string  `json:"run_key"`
	Fecha            string  `json:"fecha" format:"date"`
	UniversoDepurado int64   `json:"universo_depurado"`
	Nombre           *string `json:"nombre" nullable:"true"`
	Category         *string `json:"category" nullable:"true"`
	Rank             *int64  `json:"rank" nullable:"true" doc:"Puesto en el ranking principal (None = no estaba)."`
	EnTop15          bool    `json:"en_top15"`
	Score            opt     `json:"score" nullable:"true"`
	ScoreSimple      opt     `json:"score_simple" nullable:"true"`
	Precio           opt     `json:"precio" nullable:"true"`
	Sharpe           opt     `json:"sharpe" nullable:"true"`
	Treynor          opt     `json:"treynor" nullable:"true"`
	Beta             opt     `json:"beta" nullable:"true"`
	Perf1m           opt     `json:"perf_1m" nullable:"true"`
	Perf6m           opt     `json:"perf_6m" nullable:"true"`
	Perf1y           opt     `json:"perf_1y" nullable:"true"`
	MomentumRank     *int64  `json:"momentum_rank" nullable:"true"`
	MomentumScore    opt     `json:"momentum_score" nullable:"true"`
	ExcluidoVentana  *string `json:"excluido_ventana" nullable:"true" doc:"Si fue excluido por outlier en esa corrida."`
	ExcluidoZ        opt     `json:"excluido_z" nullable:"true"`
}

// EtfHistCategoria es una categoría en una corrida.
type EtfHistCategoria struct {
	RunKey         string `json:"run_key"`
	Fecha          string `json:"fecha" format:"date"`
	Category       string `json:"category"`
	N              *int64 `json:"n" nullable:"true"`
	ScorePromedio  opt    `json:"score_promedio" nullable:"true"`
	SharpePromedio opt    `json:"sharpe_promedio" nullable:"true"`
	Perf1yPromedio opt    `json:"perf_1y_promedio" nullable:"true"`
	EnTop15        int64  `json:"en_top15"`
}

// EtfHistPermanencia es la permanencia de un ETF en el Top 15.
type EtfHistPermanencia struct {
	Ticker      string  `json:"ticker"`
	Nombre      *string `json:"nombre" nullable:"true"`
	VecesEnTop  int64   `json:"veces_en_top"`
	Corridas    int64   `json:"corridas"`
	RachaActual int64   `json:"racha_actual"`
	RankUltima  *int64  `json:"rank_ultima" nullable:"true"`
	ScoreUltima opt     `json:"score_ultima" nullable:"true"`
}

// EtfRunResult es la respuesta de POST /run.
type EtfRunResult struct {
	Job core.JobRecord `json:"job"`
}
