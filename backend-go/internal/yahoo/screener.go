package yahoo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/loc1263/gohoo"
)

// Screener de Yahoo (POST /v1/finance/screener), el endpoint de screener de Yahoo. gohoo v0.2.1 no lo
// tiene ni expone su sesión (cookie + crumb), así que este archivo mantiene una sesión propia y mínima.
// Cuando gohoo agregue Screen este archivo se reemplaza por esa llamada.

const (
	screenBaseURL    = "https://query1.finance.yahoo.com"
	screenConsentURL = "https://fc.yahoo.com"
	screenUserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"

	// ScreenMaxSize es el máximo de resultados por página que acepta Yahoo.
	ScreenMaxSize = 250
)

// ErrEmptyScreen: Yahoo respondió sin «quotes» (cambio de API o región vacía).
var ErrEmptyScreen = errors.New("respuesta sin 'quotes' (posible cambio de API o región vacía)")

// ScreenParams es una consulta al screener.
type ScreenParams struct {
	Query     map[string]any // árbol de operadores, p. ej. EqQuery("region", "us")
	QuoteType string         // «EQUITY» (por defecto), «ETF», «MUTUALFUND»
	Offset    int
	Size      int
	SortField string
	SortAsc   bool
}

// EqQuery es el operador EQ de EquityQuery: {"operator":"EQ","operands":[campo, valor]}.
func EqQuery(field string, value any) map[string]any {
	return map[string]any{"operator": "EQ", "operands": []any{field, value}}
}

// Quotes son los resultados crudos de Yahoo (con formatted=false los números vienen sin envoltorio).
type Quotes []map[string]any

// screenSession guarda la cookie y el crumb del screener.
type screenSession struct {
	mu      sync.Mutex
	http    *http.Client
	crumb   string
	base    string
	consent string
}

func newScreenSession() *screenSession {
	jar, _ := cookiejar.New(nil)
	return &screenSession{
		http: &http.Client{Jar: jar, Timeout: 30 * time.Second},
		base: screenBaseURL, consent: screenConsentURL,
	}
}

func (s *screenSession) do(ctx context.Context, method, rawURL string, body []byte) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", screenUserAgent)
	req.Header.Set("Accept", "*/*")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		se := &gohoo.StatusError{StatusCode: resp.StatusCode, URL: strings.SplitN(rawURL, "?", 2)[0], Body: string(b[:min(len(b), 300)])}
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			var secs int
			if _, e := fmt.Sscanf(ra, "%d", &secs); e == nil && secs > 0 {
				se.RetryAfter = time.Duration(secs) * time.Second
			}
		}
		return b, se
	}
	return b, nil
}

// refresh negocia cookie y crumb (la visita a fc.yahoo.com puede dar 404 y aun así fija la cookie).
func (s *screenSession) refresh(ctx context.Context) error {
	_, _ = s.do(ctx, http.MethodGet, s.consent, nil)
	b, err := s.do(ctx, http.MethodGet, s.base+"/v1/test/getcrumb", nil)
	if err != nil {
		return fmt.Errorf("no se pudo obtener el crumb de Yahoo: %w", err)
	}
	c := strings.TrimSpace(string(b))
	if c == "" || strings.Contains(c, "<html") {
		return fmt.Errorf("crumb inválido: %q", c)
	}
	s.crumb = c
	return nil
}

func (s *screenSession) post(ctx context.Context, body []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for intento := 0; ; intento++ {
		if s.crumb == "" {
			if err := s.refresh(ctx); err != nil {
				return nil, err
			}
		}
		q := url.Values{"corsDomain": {"finance.yahoo.com"}, "formatted": {"false"}, "lang": {"en-US"}, "region": {"US"}, "crumb": {s.crumb}}
		b, err := s.do(ctx, http.MethodPost, s.base+"/v1/finance/screener?"+q.Encode(), body)
		var se *gohoo.StatusError
		if errors.As(err, &se) && se.StatusCode == http.StatusUnauthorized && intento == 0 {
			s.crumb = "" // crumb vencido: se renueva una vez
			continue
		}
		return b, err
	}
}

// Screen consulta el screener de Yahoo y devuelve los quotes crudos. Respeta el espaciado y el
// backoff ante 429 de Traffic. Una respuesta sin quotes devuelve ErrEmptyScreen.
func (c *Client) Screen(ctx context.Context, p ScreenParams) (Quotes, error) {
	if p.Size <= 0 {
		p.Size = 25
	}
	if p.Size > ScreenMaxSize {
		return nil, fmt.Errorf("Yahoo limita el screener a %d resultados por página", ScreenMaxSize)
	}
	if p.QuoteType == "" {
		p.QuoteType = "EQUITY"
	}
	sortType := "DESC"
	if p.SortAsc {
		sortType = "ASC"
	}
	body, err := json.Marshal(map[string]any{
		"offset": p.Offset, "size": p.Size, "sortField": p.SortField, "sortType": sortType,
		"userId": "", "userIdType": "guid", "query": p.Query, "quoteType": p.QuoteType,
	})
	if err != nil {
		return nil, err
	}
	c.scrMu.Lock()
	if c.scr == nil {
		c.scr = newScreenSession()
	}
	sess := c.scr
	c.scrMu.Unlock()

	var raw []byte
	err = c.T.Do(ctx, "screener", func() error {
		var e error
		raw, e = sess.post(ctx, body)
		return e
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Finance struct {
			Result []struct {
				Quotes Quotes `json:"quotes"`
			} `json:"result"`
			Error *struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
		} `json:"finance"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("respuesta del screener ilegible: %w", err)
	}
	if e := resp.Finance.Error; e != nil {
		return nil, fmt.Errorf("screener de Yahoo: %s: %s", e.Code, e.Description)
	}
	if len(resp.Finance.Result) == 0 || len(resp.Finance.Result[0].Quotes) == 0 {
		return nil, ErrEmptyScreen
	}
	return resp.Finance.Result[0].Quotes, nil
}
