package cl

import (
	"sort"

	"fogradar/internal/core"
	"fogradar/internal/transversal"
)

const metricaHub = "Score compuesto"

type hubProvider struct{}

func filasOrdenadas(s *core.Settings) (*RankingResponse, []RankingRow, error) {
	r, e := Ranking(s)
	if e != nil {
		return nil, nil, e
	}
	filas := append([]RankingRow(nil), r.Filas...)
	sort.SliceStable(filas, func(i, j int) bool {
		a, b := filas[i].Ranking, filas[j].Ranking
		if (a == nil) != (b == nil) {
			return b == nil
		}
		return a != nil && *a < *b
	})
	return r, filas, nil
}

// ResumenHub es el aporte de Chile al hub: top 3 por Score compuesto.
func (hubProvider) ResumenHub(s *core.Settings) (*transversal.ResumenModulo, error) {
	r, filas, err := filasOrdenadas(s)
	if err != nil {
		return nil, err
	}
	top := []transversal.HubItem{}
	for i := 0; i < len(filas) && i < 3; i++ {
		top = append(top, transversal.HubItem{Ticker: filas[i].Simbolo, Nombre: filas[i].Nombre, Valor: filas[i].ScoreCompuesto})
	}
	n := int64(len(filas))
	return &transversal.ResumenModulo{DatosAl: r.GeneradoEn, Metrica: metricaHub, Unidad: "numero", Top: top, Universo: &n}, nil
}

// IndiceBusqueda es el aporte de Chile al buscador global.
func (hubProvider) IndiceBusqueda(s *core.Settings) ([]transversal.EntradaBusqueda, error) {
	_, filas, err := filasOrdenadas(s)
	if err != nil {
		return nil, err
	}
	ctx := "Ranking IPSA"
	total := int64(len(filas))
	out := make([]transversal.EntradaBusqueda, 0, len(filas))
	for _, f := range filas {
		out = append(out, transversal.EntradaBusqueda{Ticker: f.Simbolo, Nombre: f.Nombre, Posicion: f.Ranking,
			Total: &total, Valor: f.ScoreCompuesto, Metrica: metricaHub, Unidad: "numero", Contexto: &ctx})
	}
	return out, nil
}

func init() { transversal.RegisterProvider(string(key), hubProvider{}) }
