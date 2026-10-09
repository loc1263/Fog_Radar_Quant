package etf

import (
	"sort"

	"fogradar/internal/core"
	"fogradar/internal/transversal"
)

const metricaHub = "Score"

type hubProvider struct{}

// ResumenHub es el aporte de ETFs al hub: top 3 del Top 15 por score.
func (hubProvider) ResumenHub(s *core.Settings) (*transversal.ResumenModulo, error) {
	top, e := Tabla(s, Top15File, "")
	if e != nil {
		return nil, e
	}
	unlock, e := BloqueoDatos(s, 0)
	if e != nil {
		return nil, e
	}
	generado := GeneradoEn(s, Top15File)
	ranking, e := Leer(s, RankingFile)
	unlock()
	if e != nil {
		return nil, e
	}
	out := &transversal.ResumenModulo{Metrica: metricaHub, Unidad: "numero", Top: []transversal.HubItem{}}
	if generado != nil {
		out.DatosAl = &core.Fecha{Time: *generado}
	}
	for i := 0; i < len(top.Filas) && i < 3; i++ {
		f := top.Filas[i]
		out.Top = append(out.Top, transversal.HubItem{Ticker: f.Ticker, Nombre: f.Nombre, Valor: f.Score})
	}
	if ranking != nil {
		n := int64(ranking.N)
		out.Universo = &n
	}
	return out, nil
}

// IndiceBusqueda: el universo depurado, en el orden del ranking principal; marca los del Top 15.
func (hubProvider) IndiceBusqueda(s *core.Settings) ([]transversal.EntradaBusqueda, error) {
	rk, e := Tabla(s, RankingFile, "")
	if e != nil {
		if e.Code == "no_data" {
			return nil, nil
		}
		return nil, e
	}
	t15, e := Tabla(s, Top15File, "")
	if e != nil {
		return nil, e
	}
	top := map[string]bool{}
	for _, f := range t15.Filas {
		top[f.Ticker] = true
	}
	filas := append([]EtfFila(nil), rk.Filas...)
	neg := func(p *float64) float64 {
		if p == nil {
			return -1e308
		}
		return *p
	}
	sort.SliceStable(filas, func(i, j int) bool { return neg(filas[i].Score) > neg(filas[j].Score) })
	total := int64(len(filas))
	out := make([]transversal.EntradaBusqueda, 0, len(filas))
	for i, f := range filas {
		pos := int64(i + 1)
		ctx := "Universo"
		if f.Categoria != nil {
			ctx = *f.Categoria
		}
		if top[f.Ticker] {
			ctx = "Top 15 · " + ctx
		}
		out = append(out, transversal.EntradaBusqueda{Ticker: f.Ticker, Nombre: f.Nombre, Posicion: &pos, Total: &total,
			Valor: f.Score, Metrica: metricaHub, Unidad: "numero", Contexto: &ctx})
	}
	return out, nil
}

func init() { transversal.RegisterProvider(string(key), hubProvider{}) }
