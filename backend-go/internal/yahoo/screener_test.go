package yahoo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/loc1263/gohoo"
)

// screenBackend es un backend falso que solo responde el screener.
type screenBackend struct {
	fakeBackend
	calls atomic.Int32
	fn    func(n int, p gohoo.ScreenParams) (*gohoo.ScreenResult, error)
}

func (s *screenBackend) Screen(_ context.Context, p gohoo.ScreenParams) (*gohoo.ScreenResult, error) {
	return s.fn(int(s.calls.Add(1)), p)
}

func TestScreenDevuelveLosQuotesCrudos(t *testing.T) {
	tr, _ := newRelojTraffic(Policy{})
	sb := &screenBackend{fn: func(_ int, p gohoo.ScreenParams) (*gohoo.ScreenResult, error) {
		if p.SortField != "intradaymarketcap" || p.Size != 5 {
			t.Errorf("parámetros: %+v", p)
		}
		return &gohoo.ScreenResult{Total: 1, Quotes: []gohoo.ScreenQuote{
			{Symbol: "AAPL", Raw: map[string]any{"symbol": "AAPL", "marketCap": 3.1e12}},
		}}, nil
	}}
	qs, err := newClientWith(tr, sb).Screen(context.Background(),
		ScreenParams{Query: gohoo.Eq("region", "us"), Size: 5, SortField: "intradaymarketcap"})
	if err != nil || len(qs) != 1 || qs[0]["symbol"] != "AAPL" || qs[0]["marketCap"] != 3.1e12 {
		t.Fatalf("%v %v", qs, err)
	}
}

func TestScreenSinQuotesEsErrEmptyScreen(t *testing.T) {
	tr, _ := newRelojTraffic(Policy{})
	sb := &screenBackend{fn: func(int, gohoo.ScreenParams) (*gohoo.ScreenResult, error) {
		return &gohoo.ScreenResult{}, nil
	}}
	if _, err := newClientWith(tr, sb).Screen(context.Background(), ScreenParams{Query: gohoo.Eq("region", "zz")}); !errors.Is(err, ErrEmptyScreen) {
		t.Errorf("err = %v", err)
	}
}

// rewrite redirige todo request (incluido fc.yahoo.com) al servidor de prueba.
type rewrite struct{ base *url.URL }

func (rw rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme, req.URL.Host = rw.base.Scheme, rw.base.Host
	return http.DefaultTransport.RoundTrip(req)
}

// Integración con el cliente real de gohoo: sesión, crumb renovado ante 401 y cuerpo del POST.
func TestScreenConGohooReal(t *testing.T) {
	var crumbs, posts atomic.Int32
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/test/getcrumb":
			_, _ = io.WriteString(w, []string{"VIEJO", "NUEVO"}[min(crumbs.Add(1)-1, 1)])
		case "/v1/finance/screener":
			posts.Add(1)
			if r.URL.Query().Get("crumb") != "NUEVO" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			b, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(b, &gotBody); err != nil {
				t.Error(err)
			}
			_, _ = io.WriteString(w, `{"finance":{"result":[{"total":1,"quotes":[{"symbol":"AAPL","marketCap":3.1e12}]}],"error":null}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	base, _ := url.Parse(srv.URL)
	tr, _ := newRelojTraffic(Policy{NetRetries: 1}) // la renovación del crumb consume un intento
	qs, err := realClient(t, tr, base).Screen(context.Background(),
		ScreenParams{Query: gohoo.Eq("region", "us"), Size: 5, SortField: "intradaymarketcap"})
	if err != nil || len(qs) != 1 || qs[0]["symbol"] != "AAPL" || posts.Load() != 2 {
		t.Fatalf("qs=%v err=%v posts=%d", qs, err, posts.Load())
	}
	if gotBody["size"] != float64(5) || gotBody["sortField"] != "intradaymarketcap" || gotBody["sortType"] != "DESC" {
		t.Errorf("cuerpo: %v", gotBody)
	}
}
