package tend

// Parámetros del modelo (config.py del original). Son constantes del módulo; el CLI de jobs puede sobrescribir
// TopN, TopNUS y MaxWeight para UNA corrida (TD-CFG-1) y quedan registrados en run_manifest.json.

// Params agrupa los parámetros ajustables y fijos del modelo.
type Params struct {
	TopN, TopNUS          int
	MaxWeight             float64
	L2Gamma               float64
	MinWeeks              int
	HorizonWeeks          int
	ShortMA, LongMA       int
	ShortMA3m, LongMA3m   int
	TopNMomentum          int
	PortfolioSizes        []int
	MinUSTickers          int
	WeeksPerYear          int
	MomentumLookbackWeeks int
}

// DefaultParams son los valores de config.py.
func DefaultParams() Params {
	return Params{
		TopN: 15, TopNUS: 15, MaxWeight: 0.15, L2Gamma: 0.15, MinWeeks: 200, HorizonWeeks: 26,
		ShortMA: 10, LongMA: 40, ShortMA3m: 4, LongMA3m: 13, TopNMomentum: 10, PortfolioSizes: []int{5, 10},
		MinUSTickers: 5, WeeksPerYear: 52, MomentumLookbackWeeks: 26,
	}
}

// Tdparametros arma el bloque «parametros» del dashboard.
func (p Params) Td() TdParametros {
	sizes := make([]int64, len(p.PortfolioSizes))
	for i, n := range p.PortfolioSizes {
		sizes[i] = int64(n)
	}
	return TdParametros{TopN: int64(p.TopN), TopNUs: int64(p.TopNUS), MaxPesoPorActivo: p.MaxWeight, L2Gamma: p.L2Gamma,
		SemanasMinimas: int64(p.MinWeeks), HorizonteSemanas: int64(p.HorizonWeeks), MaCorta: int64(p.ShortMA), MaLarga: int64(p.LongMA),
		MaCorta3m: int64(p.ShortMA3m), MaLarga3m: int64(p.LongMA3m), TopNMomentum: int64(p.TopNMomentum), TamanosPortafolio: sizes}
}
