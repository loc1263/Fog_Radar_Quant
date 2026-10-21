package cl

import (
	"encoding/json"
	"math"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/core"
	"fogradar/internal/quant"
)

// PilarKeys: nombre del pilar en el CSV → clave de la API.
var PilarKeys = map[string]string{
	"Valor": "valor", "Calidad": "calidad", "Crecimiento": "crecimiento",
	"Salud Financiera": "salud_financiera", "Dividendo": "dividendo", "Momentum": "momentum",
}

// MetricaKeys: columna del CSV → campo de RankingMetricas.
var MetricaKeys = map[string]string{
	"Trailing P/E": "trailing_pe", "Price/Book": "price_book", "Price/Sales": "price_sales",
	"Enterprise Value/EBITDA": "ev_ebitda", "Enterprise Value/Revenue": "ev_revenue",
	"Profit Margin": "profit_margin", "Operating Margin (ttm)": "operating_margin",
	"Return on Assets (ttm)": "roa", "Return on Equity (ttm)": "roe",
	"Quarterly Revenue Growth (yoy)": "revenue_growth_q", "Quarterly Earnings Growth (yoy)": "earnings_growth_q",
	"Total Debt/Equity (mrq)": "debt_equity", "Current Ratio (mrq)": "current_ratio",
	"Forward Annual Dividend Yield (%)": "dividend_yield",
	"Retorno 5D":                        "ret_5d", "Retorno 1M": "ret_1m", "Retorno 3M": "ret_3m", "Retorno 6M": "ret_6m",
	"Retorno YTD": "ret_ytd", "Retorno 1A": "ret_1a",
}

// RankingMetricas son las métricas crudas que alimentan el score (mismas unidades que el CSV).
type RankingMetricas struct {
	TrailingPE      *float64 `json:"trailing_pe" nullable:"true"`
	PriceBook       *float64 `json:"price_book" nullable:"true"`
	PriceSales      *float64 `json:"price_sales" nullable:"true"`
	EvEbitda        *float64 `json:"ev_ebitda" nullable:"true"`
	EvRevenue       *float64 `json:"ev_revenue" nullable:"true"`
	ProfitMargin    *float64 `json:"profit_margin" nullable:"true"`
	OperatingMargin *float64 `json:"operating_margin" nullable:"true"`
	Roa             *float64 `json:"roa" nullable:"true"`
	Roe             *float64 `json:"roe" nullable:"true"`
	RevenueGrowthQ  *float64 `json:"revenue_growth_q" nullable:"true"`
	EarningsGrowthQ *float64 `json:"earnings_growth_q" nullable:"true"`
	DebtEquity      *float64 `json:"debt_equity" nullable:"true"`
	CurrentRatio    *float64 `json:"current_ratio" nullable:"true"`
	DividendYield   *float64 `json:"dividend_yield" nullable:"true" doc:"En puntos porcentuales (5.1 = 5,1 %)."`
	Ret5d           *float64 `json:"ret_5d" nullable:"true"`
	Ret1m           *float64 `json:"ret_1m" nullable:"true"`
	Ret3m           *float64 `json:"ret_3m" nullable:"true"`
	Ret6m           *float64 `json:"ret_6m" nullable:"true"`
	RetYtd          *float64 `json:"ret_ytd" nullable:"true"`
	Ret1a           *float64 `json:"ret_1a" nullable:"true"`
}

// RankingRow es una fila del ranking.
type RankingRow struct {
	Ranking             *int64          `json:"ranking" nullable:"true"`
	Simbolo             string          `json:"simbolo"`
	Nombre              *string         `json:"nombre" nullable:"true"`
	Sector              *string         `json:"sector" nullable:"true"`
	Industria           *string         `json:"industria" nullable:"true"`
	ScoreCompuesto      *float64        `json:"score_compuesto" nullable:"true"`
	Pilares             PilarScores     `json:"pilares"`
	MetricasDisponibles int64           `json:"metricas_disponibles"`
	MetricasTotales     int64           `json:"metricas_totales"`
	GrupoDuplicado      *int64          `json:"grupo_duplicado" nullable:"true" doc:"Mismo id = estados financieros casi idénticos (holding/filial): riesgo de doble conteo."`
	Metricas            RankingMetricas `json:"metricas"`
}

// PilarInfo describe un pilar del modelo.
type PilarInfo struct {
	Key      string   `json:"key" enum:"valor,calidad,crecimiento,salud_financiera,dividendo,momentum"`
	Label    string   `json:"label"`
	Peso     float64  `json:"peso"`
	Metricas []string `json:"metricas"`
}

// Par es una tupla (float, float): se publica como prefixItems, como una tupla (float, float).
type Par [2]float64

// Schema implementa huma.SchemaProvider.
func (Par) Schema(huma.Registry) *huma.Schema {
	two := 2
	return &huma.Schema{Type: "array", MinItems: &two, MaxItems: &two,
		Extensions: map[string]any{"prefixItems": []any{map[string]any{"type": "number"}, map[string]any{"type": "number"}}}}
}

// ModeloInfo son los parámetros del modelo con que se generó el ranking (punto de partida de los sliders).
type ModeloInfo struct {
	Pilares []PilarInfo `json:"pilares"`
	Winsor  Par         `json:"winsor"`
}

// RankingResponse es la respuesta de /ranking.
type RankingResponse struct {
	GeneradoDesde string       `json:"generado_desde"`
	GeneradoEn    *core.Fecha  `json:"generado_en" nullable:"true"`
	Modelo        ModeloInfo   `json:"modelo"`
	Filas         []RankingRow `json:"filas"`
}

// ColumnaInfo describe una columna del catálogo.
type ColumnaInfo struct {
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	Grupo       string  `json:"grupo"`
	Unidad      string  `json:"unidad" enum:"texto,numero,monto,multiplo,pct,pct_pts,cantidad,fecha"`
	Descripcion *string `json:"descripcion" required:"false" nullable:"true"`
}

// Valor es un valor de indicador: número, texto o null.
type Valor struct{ V any }

// MarshalJSON implementa json.Marshaler.
func (v Valor) MarshalJSON() ([]byte, error) { return json.Marshal(v.V) }

// Schema implementa huma.SchemaProvider.
func (Valor) Schema(huma.Registry) *huma.Schema {
	return &huma.Schema{AnyOf: []*huma.Schema{{Type: "number"}, {Type: "string"}, {Type: "null"}}}
}

// IndicadorRow es una fila de indicadores crudos.
type IndicadorRow struct {
	Simbolo   string           `json:"simbolo"`
	Nombre    *string          `json:"nombre" nullable:"true"`
	Sector    *string          `json:"sector" nullable:"true"`
	Industria *string          `json:"industria" nullable:"true"`
	Valores   map[string]Valor `json:"valores"`
}

// IndicadoresResponse es la respuesta de /indicadores.
type IndicadoresResponse struct {
	GeneradoDesde string         `json:"generado_desde"`
	GeneradoEn    *core.Fecha    `json:"generado_en" nullable:"true"`
	Grupos        []string       `json:"grupos"`
	Columnas      []ColumnaInfo  `json:"columnas"`
	Filas         []IndicadorRow `json:"filas"`
}

// ModeloActual arma ModeloInfo desde las constantes del scoring.
func ModeloActual() ModeloInfo {
	m := ModeloInfo{Winsor: Par{WinsorInferior, WinsorSuperior}, Pilares: make([]PilarInfo, 0, len(Pilares))}
	for _, p := range Pilares {
		pi := PilarInfo{Key: PilarKeys[p], Label: p, Peso: PesosPilares[p], Metricas: []string{}}
		for _, mt := range Metrics {
			if mt.Pilar == p {
				pi.Metricas = append(pi.Metricas, MetricaKeys[mt.Columna])
			}
		}
		m.Pilares = append(m.Pilares, pi)
	}
	return m
}

func baseName(p string) string { return filepath.Base(p) }

func fnum(c *quant.Column, i int) *float64 {
	if c == nil {
		return nil
	}
	f := c.At(i)
	if math.IsNaN(f) {
		return nil
	}
	return &f
}

func fint(c *quant.Column, i int) *int64 {
	f := fnum(c, i)
	if f == nil {
		return nil
	}
	n := int64(*f)
	return &n
}

func fstr(c *quant.Column, i int) *string {
	if c == nil || c.IsNull(i) {
		return nil
	}
	s := c.Cell(i)
	return &s
}

// Ranking arma la respuesta de /ranking desde el último CSV.
func Ranking(s *core.Settings) (*RankingResponse, *core.APIError) {
	l, e := CargarRanking(s)
	if e != nil {
		return nil, e
	}
	df := l.DF
	col := df.Col
	text := func(name string) *quant.Column { return df.Col(name) }

	out := &RankingResponse{GeneradoDesde: baseName(l.Path), Modelo: ModeloActual(), Filas: make([]RankingRow, 0, df.N)}
	if l.GeneradoEn != nil {
		out.GeneradoEn = &core.Fecha{Time: *l.GeneradoEn}
	}
	for i := 0; i < df.N; i++ {
		sym := text("Símbolo").Cell(i)
		row := RankingRow{
			Ranking: fint(col("Ranking"), i), Simbolo: sym,
			Nombre: fstr(text("Nombre corto"), i), Sector: fstr(text("Sector"), i), Industria: fstr(text("Industria"), i),
			ScoreCompuesto: fnum(col("Score Compuesto"), i),
			GrupoDuplicado: fint(col("Grupo Financiero Duplicado"), i),
		}
		if v := fint(col("Métricas Disponibles"), i); v != nil {
			row.MetricasDisponibles = *v
		}
		if v := fint(col("Métricas Totales"), i); v != nil {
			row.MetricasTotales = *v
		}
		row.Pilares = PilarScores{
			Valor: fnum(col("Score Valor"), i), Calidad: fnum(col("Score Calidad"), i),
			Crecimiento: fnum(col("Score Crecimiento"), i), SaludFinanciera: fnum(col("Score Salud Financiera"), i),
			Dividendo: fnum(col("Score Dividendo"), i), Momentum: fnum(col("Score Momentum"), i),
		}
		m := func(name string) *float64 { return fnum(col(name), i) }
		row.Metricas = RankingMetricas{
			TrailingPE: m("Trailing P/E"), PriceBook: m("Price/Book"), PriceSales: m("Price/Sales"),
			EvEbitda: m("Enterprise Value/EBITDA"), EvRevenue: m("Enterprise Value/Revenue"),
			ProfitMargin: m("Profit Margin"), OperatingMargin: m("Operating Margin (ttm)"),
			Roa: m("Return on Assets (ttm)"), Roe: m("Return on Equity (ttm)"),
			RevenueGrowthQ: m("Quarterly Revenue Growth (yoy)"), EarningsGrowthQ: m("Quarterly Earnings Growth (yoy)"),
			DebtEquity: m("Total Debt/Equity (mrq)"), CurrentRatio: m("Current Ratio (mrq)"),
			DividendYield: m("Forward Annual Dividend Yield (%)"),
			Ret5d:         m("Retorno 5D"), Ret1m: m("Retorno 1M"), Ret3m: m("Retorno 3M"), Ret6m: m("Retorno 6M"),
			RetYtd: m("Retorno YTD"), Ret1a: m("Retorno 1A"),
		}
		out.Filas = append(out.Filas, row)
	}
	return out, nil
}

// Indicadores arma la respuesta de /indicadores desde el último CSV de indicadores.
func Indicadores(s *core.Settings) (*IndicadoresResponse, *core.APIError) {
	l, e := CargarIndicadores(s)
	if e != nil {
		return nil, e
	}
	df := l.DF
	var presentes []Columna
	for _, c := range Columnas {
		if df.Has(c.CSV) {
			presentes = append(presentes, c)
		}
	}
	out := &IndicadoresResponse{GeneradoDesde: baseName(l.Path), Grupos: []string{},
		Columnas: make([]ColumnaInfo, 0, len(presentes)), Filas: make([]IndicadorRow, 0, df.N)}
	if l.GeneradoEn != nil {
		out.GeneradoEn = &core.Fecha{Time: *l.GeneradoEn}
	}
	for _, g := range Grupos {
		for _, c := range presentes {
			if c.Grupo == g {
				out.Grupos = append(out.Grupos, g)
				break
			}
		}
	}
	for _, c := range presentes {
		out.Columnas = append(out.Columnas, ColumnaInfo{Key: c.Key, Label: c.Label, Grupo: c.Grupo, Unidad: c.Unidad, Descripcion: c.Descripcion})
	}
	for i := 0; i < df.N; i++ {
		r := IndicadorRow{Simbolo: df.Col("Símbolo").Cell(i), Nombre: fstr(df.Col("Nombre corto"), i),
			Sector: fstr(df.Col("Sector"), i), Industria: fstr(df.Col("Industria"), i),
			Valores: make(map[string]Valor, len(presentes))}
		for _, c := range presentes {
			r.Valores[c.Key] = Valor{df.Col(c.CSV).Value(i)}
		}
		out.Filas = append(out.Filas, r)
	}
	return out, nil
}

// PilarScores es el z-score promedio de cada pilar (null si no hay métricas en ese pilar).
type PilarScores struct {
	Valor           *float64 `json:"valor" nullable:"true"`
	Calidad         *float64 `json:"calidad" nullable:"true"`
	Crecimiento     *float64 `json:"crecimiento" nullable:"true"`
	SaludFinanciera *float64 `json:"salud_financiera" nullable:"true"`
	Dividendo       *float64 `json:"dividendo" nullable:"true"`
	Momentum        *float64 `json:"momentum" nullable:"true"`
}
