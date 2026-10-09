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

// IndiceBusqueda: acciones de los portafolios (cada análisis) y líderes de momentum de la última corrida.
func (hubProvider) IndiceBusqueda(s *core.Settings) ([]transversal.EntradaBusqueda, error) {
	p, err := ultima(s)
	if err != nil {
		return nil, err
	}
	var out []transversal.EntradaBusqueda
	vistos := map[string]bool{}
	for _, a := range p.Analyses {
		hs := byPeso(a.Holdings)
		ctx := "Portafolio " + a.Label
		total := int64(len(hs))
		for i, h := range hs {
			pos, v := int64(i+1), h.PesoFinal
			out = append(out, transversal.EntradaBusqueda{Ticker: h.Ticker, Nombre: h.Nombre, Posicion: &pos, Total: &total,
				Valor: &v, Metrica: metricaHub, Unidad: "fraccion", Contexto: &ctx})
			vistos[h.Ticker] = true
		}
		for _, m := range a.Momentum {
			if vistos[m.Ticker] {
				continue
			}
			vistos[m.Ticker] = true
			rank := m.Rank
			var v *float64
			if m.MomentumScorePct != nil {
				x := *m.MomentumScorePct / 100
				v = &x
			}
			c := "Momentum mensual " + a.Label
			out = append(out, transversal.EntradaBusqueda{Ticker: m.Ticker, Nombre: m.Nombre, Posicion: &rank, Valor: v,
				Metrica: "Score de momentum", Unidad: "fraccion", Contexto: &c})
		}
	}
	return out, nil
}

func init() { transversal.RegisterProvider(string(key), hubProvider{}) }
