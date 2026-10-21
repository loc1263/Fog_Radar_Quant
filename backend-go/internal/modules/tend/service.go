package tend

import (
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
	d.MarketRegimeNotes, d.Excluidos = nz(d.MarketRegimeNotes), nz(d.Excluidos)
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

// Dashboard sirve la última corrida completa.
func Dashboard(s *core.Settings) (*TdDashboard, *core.APIError) {
	d, e := LatestRun(s)
	if e != nil {
		return nil, e
	}
	var out TdDashboard
	if e := ReadDashboard(d, &out); e != nil {
		return nil, e
	}
	out.Parametros = core.Some(DefaultParams().Td())
	out.Excluidos = Excluidos(s) // el archivo se reescribe en cada armado del dataset
	out.normalize()
	return &out, nil
}
