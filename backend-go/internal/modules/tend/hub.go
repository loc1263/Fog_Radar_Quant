package tend

import (
	"sort"

	"fogradar/internal/core"
	"fogradar/internal/transversal"
)

const metricaHub = "Peso en el portafolio"

type hubProvider struct{}

func ultima(s *core.Settings) (*TdDashboard, error) {
	d, e := LatestRun(s)
	if e != nil {
		return nil, e
	}
	var out TdDashboard
	if e := ReadDashboard(d, &out); e != nil {
		return nil, e
	}
	out.normalize()
	return &out, nil
}

func byPeso(h []TdHolding) []TdHolding {
	out := append([]TdHolding(nil), h...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].PesoFinal > out[j].PesoFinal })
	return out
}

// ResumenHub es el aporte de Tendencia al hub: top 3 del portafolio principal (última corrida).
func (hubProvider) ResumenHub(s *core.Settings) (*transversal.ResumenModulo, error) {
	p, err := ultima(s)
	if err != nil {
		return nil, err
	}
	var datosAl *core.Fecha
	if !p.GeneratedAt.IsZero() {
		g := p.GeneratedAt
		datosAl = &g
	}
	out := &transversal.ResumenModulo{DatosAl: datosAl, Metrica: metricaHub, Unidad: "fraccion", Top: []transversal.HubItem{}}
	if len(p.Analyses) > 0 {
		a := p.Analyses[0]
		for i, h := range byPeso(a.Holdings) {
			if i >= 3 {
				break
			}
			v := h.PesoFinal
			out.Top = append(out.Top, transversal.HubItem{Ticker: h.Ticker, Nombre: h.Nombre, Valor: &v})
		}
		u := a.UniverseSize
		out.Universo = &u
	}
	return out, nil
}

func init() { transversal.RegisterProvider(string(key), hubProvider{}) }
