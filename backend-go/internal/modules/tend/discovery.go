package tend

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/loc1263/gohoo"

	"fogradar/internal/fx"
	"fogradar/internal/yahoo"
)

// Descubrimiento dinámico del universo: screener de Yahoo por región,
// ranking por capitalización, deduplicación por empresa, sector y sede vía quoteSummary/assetProfile.

const (
	// UniverseMaxAgeDays: un snapshot con más días que esto se vuelve a descubrir.
	UniverseMaxAgeDays = 3

	discCandidatesPerRegion = 20
	discAsiaTopN            = 20
	discUSRegion            = "us"
	discUSMegacapMin        = 200_000_000_000.0 // USD; todas las mega-cap, no un top-N fijo
	discUSMegacapPool       = 150
	discUSHQCountry         = "United States"
	discMinViable           = 10
)

// discWorkers son las consultas de quoteSummary en vuelo a la vez si el llamador no indica otra cifra. El espaciado
// global de Yahoo sigue fijando la tasa de requests: los workers solo ocultan la latencia.
const discWorkers = 4

// profileRetryWait es la espera entre los 2 intentos de quoteSummary de un ticker; var para los tests.
var profileRetryWait = 2 * time.Second

// screenBackoff es la espera base entre reintentos del screener (3 s, 6 s…); var para los tests.
var screenBackoff = 3 * time.Second

// asiaRegions conserva el orden de DISCOVERY_ASIA_REGIONS.
var asiaRegions = []struct{ Code, Country string }{
	{"cn", "China"}, {"hk", "China (Hong Kong)"}, {"jp", "Japón"}, {"kr", "Corea del Sur"}, {"tw", "Taiwán"}, {"in", "India"},
}

// Screener es lo que el descubrimiento necesita de Yahoo (lo cumple *yahoo.Client).
type Screener interface {
	Screen(ctx context.Context, p yahoo.ScreenParams) (yahoo.Quotes, error)
	QuoteSummary(ctx context.Context, ticker string, modules []string) (map[string]any, error)
}

// candidate es un quote normalizado del screener.
type candidate struct {
	Symbol, Name, Country, Currency string
	MarketCap                       float64
	Sector                          string
	HQCountry                       *string // sede real (solo EEUU); nil = desconocida
	hqKnown                         bool
}

func quoteStr(q map[string]any, key string) string {
	s, _ := q[key].(string)
	return s
}

// quoteValue lee un número que puede venir envuelto en {"raw": x, "fmt": "..."}.
func quoteValue(q map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		v, ok := q[k]
		if !ok || v == nil {
			continue
		}
		if m, isMap := v.(map[string]any); isMap {
			v = m["raw"]
		}
		f, ok := gohoo.AsFloat(v)
		return f, ok
	}
	return 0, false
}

func quoteSector(q map[string]any) string {
	for _, k := range []string{"sectorDisp", "sector", "industryDisp", "industry"} {
		v := q[k]
		if m, ok := v.(map[string]any); ok {
			if s, _ := m["raw"].(string); s != "" {
				return s
			}
			v = m["fmt"]
		}
		if s, _ := v.(string); s != "" {
			return s
		}
	}
	return "Otro"
}

// normalizeQuote devuelve nil si el quote no sirve (sin símbolo, capitalización, moneda o no es acción).
func normalizeQuote(q map[string]any, country string) *candidate {
	symbol := quoteStr(q, "symbol")
	if symbol == "" {
		return nil
	}
	name := symbol
	for _, k := range []string{"longName", "shortName", "displayName"} {
		if s := quoteStr(q, k); s != "" {
			name = s
			break
		}
	}
	mc, ok := quoteValue(q, "marketCap", "intradaymarketcap")
	currency := quoteStr(q, "currency")
	if !ok || currency == "" {
		return nil
	}
	if qt, present := q["quoteType"]; present && qt != nil && qt != "EQUITY" {
		return nil
	}
	return &candidate{Symbol: symbol, Name: name, Country: country, Currency: currency, MarketCap: mc, Sector: quoteSector(q)}
}

var nameSuffixRE = regexp.MustCompile(`\b(LTD|LIMITED|CORPORATION|CORP|CO|INC|HOLDINGS|HOLDING|GROUP|CLASS\s+[AH]|\(ADR\))\b`)
var punctRE = regexp.MustCompile(`[.,]`)
var spacesRE = regexp.MustCompile(`\s+`)

// normName normaliza un nombre de empresa: mayúsculas, sin sufijos societarios (con \b de Unicode), sin «.,»,
// espacios colapsados y recortado a 20 caracteres. RE2 solo conoce \b ASCII, así que se busca sobre una
// copia con un byte por carácter (las letras y dígitos no ASCII pasan a «X») y se recorta el original.
func normName(n string) string {
	rs := []rune(strings.ToUpper(n))
	m := make([]byte, len(rs))
	for i, r := range rs {
		switch {
		case r < 128:
			m[i] = byte(r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			m[i] = 'X'
		default:
			m[i] = '#'
		}
	}
	var out []rune
	last := 0
	for _, loc := range nameSuffixRE.FindAllIndex(m, -1) {
		out = append(out, rs[last:loc[0]]...)
		last = loc[1]
	}
	out = append(out, rs[last:]...)
	s := punctRE.ReplaceAllString(string(out), "")
	s = strings.TrimSpace(spacesRE.ReplaceAllString(s, " "))
	if r := []rune(s); len(r) > 20 {
		s = string(r[:20])
	}
	return s
}

// dedupeByCompany se queda con el listado de mayor capitalización de cada empresa (orden de primera aparición).
func dedupeByCompany(cs []*candidate) []*candidate {
	var order []string
	best := map[string]*candidate{}
	for _, c := range cs {
		k := normName(c.Name)
		cur, ok := best[k]
		if !ok {
			order = append(order, k)
		}
		if !ok || c.MarketCap > cur.MarketCap {
			best[k] = c
		}
	}
	out := make([]*candidate, len(order))
	for i, k := range order {
		out[i] = best[k]
	}
	return out
}

// selectCandidates ordena por capitalización descendente. Con minMarketCap > 0 deja todas las que lo superan;
// si no, las topN primeras.
func selectCandidates(cs []*candidate, topN int, minMarketCap float64) []*candidate {
	ranked := append([]*candidate{}, cs...)
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].MarketCap > ranked[j].MarketCap })
	if minMarketCap > 0 {
		var out []*candidate
		for _, c := range ranked {
			if c.MarketCap >= minMarketCap {
				out = append(out, c)
			}
		}
		return out
	}
	return ranked[:min(topN, len(ranked))]
}

// excludeForeignHQ saca las empresas con sede conocida fuera de EEUU (ADR de empresas extranjeras). Sin dato se conservan.
func excludeForeignHQ(cs []*candidate, log func(string, ...any)) []*candidate {
	var out []*candidate
	var foreign []string
	for _, c := range cs {
		if c.HQCountry != nil && *c.HQCountry != discUSHQCountry {
			foreign = append(foreign, fmt.Sprintf("%s (%s)", c.Symbol, *c.HQCountry))
			continue
		}
		out = append(out, c)
	}
	if len(foreign) > 0 {
		log("[EEUU] excluidas %d empresas con sede fuera de EEUU (ADR/instrumento en USD de una empresa extranjera): %s", len(foreign), strings.Join(foreign, ", "))
	}
	return out
}

type discoverer struct {
	src     Screener
	info    func(string, ...any)
	warn    func(string, ...any)
	cache   *SectorCache // nil = sin caché
	workers int
}

// screenRegion pide las `size` mayores empresas de una región, con 3 intentos. Sin resultado devuelve nil.
func (d *discoverer) screenRegion(ctx context.Context, region string, size int) yahoo.Quotes {
	const maxRetries = 3
	var last error
	for attempt := range maxRetries {
		qs, err := d.src.Screen(ctx, yahoo.ScreenParams{
			Query: gohoo.Eq("region", region), Size: size, SortField: "intradaymarketcap",
		})
		if err == nil {
			return qs
		}
		last = err
		d.warn("[región %s] intento %d/%d falló: %v", region, attempt+1, maxRetries, err)
		if ctx.Err() != nil {
			return nil
		}
		if attempt < maxRetries-1 && !sleepCtx(ctx, screenBackoff<<attempt) {
			return nil
		}
	}
	d.warn("[región %s] descubrimiento falló tras %d intentos: %v", region, maxRetries, last)
	return nil
}

func sleepCtx(ctx context.Context, dur time.Duration) bool {
	t := time.NewTimer(dur)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// profile consulta assetProfile (2 intentos) y devuelve el sector y la sede; ok=false si ambos fallaron.
func (d *discoverer) profile(ctx context.Context, label, symbol string) (sector, country string, ok bool) {
	for attempt := range 2 {
		qs, err := d.src.QuoteSummary(ctx, symbol, []string{"assetProfile"})
		if err == nil {
			ap, _ := qs["assetProfile"].(map[string]any)
			info := yahoo.FlattenInfo(map[string]any{"assetProfile": ap})
			country, _ = info["country"].(string)
			return firstStr(info, "sector", "industry", "Otro"), country, true
		}
		d.warn("[%s] [sector] %s: intento %d/2 falló: %v", label, symbol, attempt+1, err)
		if ctx.Err() != nil || (attempt == 0 && !sleepCtx(ctx, profileRetryWait)) {
			break
		}
	}
	return "Otro", "", false
}

// enrich completa sector (y sede si captureHQ) de los candidatos pendientes: primero desde la caché y el resto con
// quoteSummary/assetProfile en paralelo (el espaciado global de Yahoo limita la tasa).
func (d *discoverer) enrich(ctx context.Context, cs []*candidate, label string, captureHQ bool) {
	var live []*candidate
	pending, hits := 0, 0
	for _, c := range cs {
		if c.Sector != "Otro" && (!captureHQ || c.hqKnown) {
			continue
		}
		pending++
		if e, ok := d.cache.get(c.Symbol); ok {
			c.Sector = e.Sector
			if captureHQ && e.Country != "" {
				country := e.Country
				c.HQCountry = &country
			}
			c.hqKnown = true
			hits++
			continue
		}
		live = append(live, c)
	}
	if pending == 0 {
		return
	}
	d.info("[%s] sector/sede de %d tickers: %d desde la caché, %d vía quoteSummary…", label, pending, hits, len(live))
	if len(live) == 0 {
		return
	}

	var mu sync.Mutex
	done, otro := 0, 0
	work := make(chan *candidate)
	var wg sync.WaitGroup
	for range max(1, min(d.workers, len(live))) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range work {
				sector, country, ok := d.profile(ctx, label, c.Symbol)
				c.Sector = sector
				if captureHQ && country != "" {
					c.HQCountry = &country
				}
				c.hqKnown = true // si fallaron las 2 llamadas queda sin sede (se conserva)
				if ok && sector != "Otro" {
					d.cache.put(c.Symbol, sector, country)
				}
				mu.Lock()
				done++
				if sector == "Otro" {
					otro++
				}
				d.info("[%s] [sector] (%d/%d) %s: %s", label, done, len(live), c.Symbol, sector)
				mu.Unlock()
			}
		}()
	}
feed:
	for _, c := range live {
		select {
		case work <- c:
		case <-ctx.Done():
			break feed
		}
	}
	close(work)
	wg.Wait()
	if ctx.Err() != nil {
		return
	}
	if otro > 0 {
		if float64(otro)/float64(len(cs)) > 0.3 {
			d.warn("[%s] %d/%d tickers quedaron con sector 'Otro' incluso tras consultar quoteSummary: posible bloqueo/rate-limit persistente o cambio de esquema.", label, otro, len(cs))
		} else {
			d.info("[%s] %d/%d tickers quedaron sin sector (probablemente un rate-limit puntual).", label, otro, len(cs))
		}
	}
}

// firstStr devuelve el primer valor no vacío entre las claves; la última «clave» es el valor por defecto.
func firstStr(m map[string]any, keys ...string) string {
	for _, k := range keys[:len(keys)-1] {
		if s, _ := m[k].(string); s != "" {
			return s
		}
	}
	return keys[len(keys)-1]
}

func (d *discoverer) asia(ctx context.Context) []*candidate {
	var cs []*candidate
	for _, r := range asiaRegions {
		qs := d.screenRegion(ctx, r.Code, discCandidatesPerRegion)
		for _, q := range qs {
			if c := normalizeQuote(q, r.Country); c != nil {
				cs = append(cs, c)
			}
		}
		d.info("[Asia] región '%s' (%s): %d resultados crudos", r.Code, r.Country, len(qs))
	}
	if len(cs) == 0 {
		return nil
	}
	top := selectCandidates(dedupeByCompany(cs), discAsiaTopN, 0)
	d.enrich(ctx, top, "Asia", false)
	return top
}

func (d *discoverer) us(ctx context.Context) []*candidate {
	var cs []*candidate
	for _, q := range d.screenRegion(ctx, discUSRegion, max(discUSMegacapPool, discCandidatesPerRegion)) {
		if c := normalizeQuote(q, "EEUU"); c != nil {
			cs = append(cs, c)
		}
	}
	cs = dedupeByCompany(cs)
	top := selectCandidates(cs, 0, discUSMegacapMin)
	d.info("[EEUU] modo mega-cap (capitalización >= $%.0f): %d empresas de %d candidatas evaluadas", discUSMegacapMin, len(top), len(cs))
	d.enrich(ctx, top, "EEUU", true)
	return excludeForeignHQ(top, d.warn)
}

// DiscoverOpts son los ajustes opcionales del descubrimiento.
type DiscoverOpts struct {
	Cache   *SectorCache // sector y sede ya conocidos (nil = consultar todo)
	Workers int          // consultas de quoteSummary en paralelo (0 = discWorkers)
}

// DiscoverUniverse arma el universo dinámico (Asia + mega-cap de EEUU). Falla si el screener devuelve muy pocas.
func DiscoverUniverse(ctx context.Context, src Screener, info, warn func(string, ...any)) (*Universe, error) {
	return DiscoverUniverseOpts(ctx, src, DiscoverOpts{}, info, warn)
}

// DiscoverUniverseOpts es DiscoverUniverse con caché de sectores y paralelismo configurables.
func DiscoverUniverseOpts(ctx context.Context, src Screener, o DiscoverOpts, info, warn func(string, ...any)) (*Universe, error) {
	workers := o.Workers
	if workers <= 0 {
		workers = discWorkers
	}
	d := &discoverer{src: src, info: info, warn: warn, cache: o.Cache, workers: workers}
	defer func() { // lo consultado se guarda aunque el descubrimiento falle después
		if err := o.Cache.Save(); err != nil {
			warn("no se pudo guardar la caché de sectores: %v", err)
		}
	}()
	info("Descubriendo universo asiático (China, Hong Kong, Japón, Corea, Taiwán, India)…")
	asia := d.asia(ctx)
	info("Universo asiático descubierto: %d empresas (objetivo %d)", len(asia), discAsiaTopN)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	info("Descubriendo universo EEUU…")
	us := d.us(ctx)
	info("Universo EEUU descubierto: %d empresas (todas las mega-cap, tamaño variable)", len(us))
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if len(asia) < discMinViable || len(us) < discMinViable {
		return nil, fmt.Errorf("descubrimiento insuficiente (Asia=%d, EEUU=%d)", len(asia), len(us))
	}
	u := &Universe{Info: map[string]Info{}, Sector: map[string]string{}, Source: "dynamic_yahoo_screener"}
	add := func(cs []*candidate, dst *[]string) {
		for _, c := range cs {
			u.Sector[c.Symbol] = c.Sector
			if _, _, ok := fx.Normalize(c.Currency); !ok {
				warn("  excluido %s (%s): moneda '%s' sin par FX configurado", c.Symbol, c.Name, c.Currency)
				continue
			}
			*dst = append(*dst, c.Symbol)
			u.Info[c.Symbol] = Info{c.Name, c.Country, c.Currency}
		}
	}
	add(asia, &u.Asia)
	add(us, &u.US)
	if len(u.Asia) == 0 || len(u.US) == 0 {
		return nil, errors.New("el universo descubierto quedó vacío tras filtrar monedas")
	}
	return u, nil
}
