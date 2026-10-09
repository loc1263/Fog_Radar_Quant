package yahoo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(base string) *Client {
	c := newClientWith(NewTraffic(Policy{Concurrency: 1}), nil)
	s := newScreenSession()
	s.base, s.consent = base, base+"/consent"
	c.scr = s
	return c
}

func TestScreenPideCrumbYPublicaElCuerpo(t *testing.T) {
	var crumbs atomic.Int32
	var gotBody map[string]any
	var gotQuery map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/consent":
			http.SetCookie(w, &http.Cookie{Name: "A3", Value: "x"})
			w.WriteHeader(404)
		case "/v1/test/getcrumb":
			crumbs.Add(1)
			io.WriteString(w, "CRUMB1")
		case "/v1/finance/screener":
			gotQuery = r.URL.Query()
			if c, _ := r.Cookie("A3"); c == nil {
				t.Error("falta la cookie de sesión")
			}
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, &gotBody)
			io.WriteString(w, `{"finance":{"result":[{"quotes":[{"symbol":"AAPL","marketCap":3.1e12,"currency":"USD"}]}],"error":null}}`)
		}
	}))
	defer srv.Close()
	c := testClient(srv.URL)
	qs, err := c.Screen(context.Background(), ScreenParams{Query: EqQuery("region", "us"), Size: 5, SortField: "intradaymarketcap"})
	if err != nil || len(qs) != 1 || qs[0]["symbol"] != "AAPL" {
		t.Fatalf("%v %v", qs, err)
	}
	if gotQuery["crumb"][0] != "CRUMB1" || gotQuery["formatted"][0] != "false" {
		t.Errorf("query: %v", gotQuery)
	}
	if gotBody["sortType"] != "DESC" || gotBody["quoteType"] != "EQUITY" || gotBody["size"] != float64(5) || gotBody["sortField"] != "intradaymarketcap" {
		t.Errorf("cuerpo: %v", gotBody)
	}
	q := gotBody["query"].(map[string]any)
	if q["operator"] != "EQ" {
		t.Errorf("query: %v", q)
	}
	// Segunda llamada: reutiliza el crumb.
	if _, err := c.Screen(context.Background(), ScreenParams{Query: EqQuery("region", "jp"), Size: 5}); err != nil || crumbs.Load() != 1 {
		t.Fatalf("crumbs=%d err=%v", crumbs.Load(), err)
	}
}

func TestScreenRenuevaElCrumbEn401(t *testing.T) {
	var crumbs, posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/test/getcrumb":
			io.WriteString(w, []string{"VIEJO", "NUEVO"}[min(crumbs.Add(1)-1, 1)])
		case "/v1/finance/screener":
			posts.Add(1)
			if r.URL.Query().Get("crumb") != "NUEVO" {
				w.WriteHeader(401)
				return
			}
			io.WriteString(w, `{"finance":{"result":[{"quotes":[{"symbol":"X"}]}]}}`)
		}
	}))
	defer srv.Close()
	if _, err := testClient(srv.URL).Screen(context.Background(), ScreenParams{Query: EqQuery("region", "us")}); err != nil || posts.Load() != 2 {
		t.Fatalf("posts=%d err=%v", posts.Load(), err)
	}
}

func TestScreenErrores(t *testing.T) {
	var cuerpo atomic.Value
	cuerpo.Store(`{"finance":{"result":[{"quotes":[]}]}}`)
	var status atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/test/getcrumb" {
			io.WriteString(w, "C")
			return
		}
		if st := int(status.Load()); st != 0 {
			w.WriteHeader(st)
			return
		}
		io.WriteString(w, cuerpo.Load().(string))
	}))
	defer srv.Close()
	c := testClient(srv.URL)
	p := ScreenParams{Query: EqQuery("region", "us")}
	if _, err := c.Screen(context.Background(), p); !errors.Is(err, ErrEmptyScreen) {
		t.Errorf("sin quotes: %v", err)
	}
	cuerpo.Store(`{"finance":{"result":null,"error":{"code":"Bad Request","description":"campo inválido"}}}`)
	if _, err := c.Screen(context.Background(), p); err == nil {
		t.Error("error de Yahoo debía propagarse")
	}
	if _, err := c.Screen(context.Background(), ScreenParams{Query: p.Query, Size: ScreenMaxSize + 1}); err == nil {
		t.Error("size > 250 debía rechazarse")
	}
	status.Store(500)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := c.Screen(ctx, p); err == nil {
		t.Error("HTTP 500 debía fallar")
	}
}
