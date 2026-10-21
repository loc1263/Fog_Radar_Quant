package tend

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Info de un ticker del universo: nombre, país y moneda de cotización.
type Info struct{ Name, Country, Currency string }

// Universe es el universo de tickers (Asia + EEUU) con orden estable, como un diccionario ordenado.
type Universe struct {
	Asia, US []string // tickers en orden
	Info     map[string]Info
	Sector   map[string]string
	Source   string
	At       string
}

// All devuelve Asia seguida de EEUU (ALL_TICKERS).
func (u *Universe) All() []string { return append(append([]string{}, u.Asia...), u.US...) }

// Has dice si el ticker está en la lista dada.
func has(list []string, t string) bool {
	for _, x := range list {
		if x == t {
			return true
		}
	}
	return false
}

// SectorOf devuelve el sector de un ticker («Otro» si no está).
func (u *Universe) SectorOf(t string) string {
	if s, ok := u.Sector[t]; ok {
		return s
	}
	return "Otro"
}

// orderedObject decodifica un objeto JSON conservando el orden de las claves.
func orderedObject(raw json.RawMessage) ([]string, map[string]json.RawMessage, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, nil, fmt.Errorf("se esperaba un objeto JSON")
	}
	var keys []string
	vals := map[string]json.RawMessage{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, nil, err
		}
		k := kt.(string)
		keys = append(keys, k)
		vals[k] = v
	}
	return keys, vals, nil
}

// ParseUniverse lee un snapshot universe_<fecha>.json (asia_tickers/us_tickers: ticker → [nombre, país, moneda]).
func ParseUniverse(data []byte) (*Universe, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, err
	}
	u := &Universe{Info: map[string]Info{}, Sector: map[string]string{}, Source: "dynamic_yahoo_screener"}
	read := func(key string) ([]string, error) {
		keys, vals, err := orderedObject(top[key])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		for _, k := range keys {
			var v [3]string
			if err := json.Unmarshal(vals[k], &v); err != nil {
				return nil, fmt.Errorf("%s[%s]: %w", key, k, err)
			}
			u.Info[k] = Info{v[0], v[1], v[2]}
		}
		return keys, nil
	}
	var err error
	if u.Asia, err = read("asia_tickers"); err != nil {
		return nil, err
	}
	if u.US, err = read("us_tickers"); err != nil {
		return nil, err
	}
	if len(u.Asia) == 0 || len(u.US) == 0 {
		return nil, fmt.Errorf("snapshot de universo vacío")
	}
	if raw, ok := top["sector_map"]; ok {
		if err := json.Unmarshal(raw, &u.Sector); err != nil {
			return nil, err
		}
	}
	if raw, ok := top["method"]; ok {
		_ = json.Unmarshal(raw, &u.Source)
	}
	if raw, ok := top["discovered_at"]; ok {
		_ = json.Unmarshal(raw, &u.At)
	}
	return u, nil
}

// LatestUniverseFile devuelve el snapshot de hoy si existe y si no el más reciente ("" si no hay).
func LatestUniverseFile(dir string, hoy time.Time) string {
	files, _ := filepath.Glob(filepath.Join(dir, "universe_*.json"))
	if len(files) == 0 {
		return ""
	}
	sort.Strings(files)
	tag := hoy.Format("20060102")
	var todays []string
	for _, f := range files {
		if strings.Contains(filepath.Base(f), tag) {
			todays = append(todays, f)
		}
	}
	if len(todays) > 0 {
		return todays[len(todays)-1]
	}
	return files[len(files)-1]
}

// Prices es la matriz de precios semanales en USD: una columna por ticker, filas por fecha.
type Prices struct {
	Dates   []time.Time
	Tickers []string
	Cols    [][]float64 // Cols[j][i] = precio del ticker j en la fecha i (NaN si falta)
}

// Col devuelve la serie de un ticker (nil si no está).
func (p *Prices) Col(t string) []float64 {
	for j, x := range p.Tickers {
		if x == t {
			return p.Cols[j]
		}
	}
	return nil
}

// Subset devuelve las columnas dadas, en ese orden.
func (p *Prices) Subset(tickers []string) *Prices {
	out := &Prices{Dates: p.Dates, Tickers: tickers}
	for _, t := range tickers {
		out.Cols = append(out.Cols, p.Col(t))
	}
	return out
}
