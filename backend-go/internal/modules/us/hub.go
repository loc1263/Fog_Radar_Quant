package us

import (
	"sort"

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

// IndiceBusqueda: los candidatos evaluados, ordenados por momentum (la posición del Top 15 sale de ahí).
func (hubProvider) IndiceBusqueda(s *core.Settings) ([]transversal.EntradaBusqueda, error) {
	dir, e := LatestFinishedRun(s)
	if e != nil {
		return nil, e
	}
	var res RunResponse
	if e := ReadRunFile(dir, "result.json", &res); e != nil {
		return nil, e
	}
	top := map[string]bool{}
	for _, r := range res.Rows {
		top[r.Ticker] = true
	}
	filas := res.Rows
	var cand RunResponse
	if ReadRunFile(dir, "candidates.json", &cand) == nil && cand.Rows != nil {
		filas = cand.Rows // corridas sin candidates.json: solo el Top 15
	}
	filas = append([]CandidateRow(nil), filas...)
	sort.SliceStable(filas, func(i, j int) bool { return filas[i].Momentum121 > filas[j].Momentum121 })
	total := int64(len(filas))
	out := make([]transversal.EntradaBusqueda, 0, len(filas))
	for i, r := range filas {
		pos, m := int64(i+1), r.Momentum121
		ctx := "Candidatos"
		if top[r.Ticker] {
			ctx = "Top 15"
		}
		out = append(out, transversal.EntradaBusqueda{Ticker: r.Ticker, Nombre: r.Name, Posicion: &pos, Total: &total,
			Valor: &m, Metrica: metricaHub, Unidad: "fraccion", Contexto: &ctx})
	}
	return out, nil
}

func init() { transversal.RegisterProvider(string(key), hubProvider{}) }
