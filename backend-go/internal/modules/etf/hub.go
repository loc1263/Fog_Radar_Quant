package etf

import (
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

func init() { transversal.RegisterProvider(string(key), hubProvider{}) }
