package us

import (
	"fogradar/internal/core"
	"fogradar/internal/transversal"
)

const metricaHub = "Momentum 12-1"

type hubProvider struct{}

// ResumenHub es el aporte de EEUU al hub (última corrida terminada): top 3 por momentum.
func (hubProvider) ResumenHub(s *core.Settings) (*transversal.ResumenModulo, error) {
	dir, e := LatestFinishedRun(s)
	if e != nil {
		return nil, e
	}
	var r RunResponse
	if e := ReadRunFile(dir, "result.json", &r); e != nil {
		return nil, e
	}
	fin := r.FinishedAt
	top := []transversal.HubItem{}
	for i := 0; i < len(r.Rows) && i < 3; i++ {
		m := r.Rows[i].Momentum121
		top = append(top, transversal.HubItem{Ticker: r.Rows[i].Ticker, Nombre: r.Rows[i].Name, Valor: &m})
	}
	n := r.CandidatesTotal
	return &transversal.ResumenModulo{DatosAl: &fin, Metrica: metricaHub, Unidad: "fraccion", Top: top, Universo: &n}, nil
}

func init() { transversal.RegisterProvider(string(key), hubProvider{}) }
