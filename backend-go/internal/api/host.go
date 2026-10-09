// Package api arma el servidor HTTP: huma sobre chi, con el contrato público de la API
// (rutas, envelope de error {detail, code}, CORS, rate limit y API key).
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"runtime/debug"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"fogradar/internal/core"
)

const securityScheme = "APIKeyHeader"

// Host reúne lo que comparten todos los routers: configuración, limitador y registro de jobs.
// Cada módulo registra sus endpoints con Public/Protected.
type Host struct {
	S      *core.Settings
	Lim    *core.Limiter
	Jobs   *core.JobRegistry
	API    huma.API
	Router chi.Router
	Log    *slog.Logger
}

func init() {
	huma.DefaultArrayNullable = false // las listas siempre son [] (nunca null)
	// Los errores generados por huma (validación, parseo) usan el envelope de error de la API.
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		detail := msg
		if status == http.StatusUnprocessableEntity && len(errs) > 0 {
			if d, ok := errs[0].(huma.ErrorDetailer); ok {
				ed := d.ErrorDetail()
				loc := strings.TrimPrefix(ed.Location, "body.")
				if loc != "" {
					detail = loc + ": " + ed.Message
				} else {
					detail = ed.Message
				}
			}
		}
		code := core.CodeForStatus(status)
		if status == http.StatusUnprocessableEntity {
			code = "validation_error"
		}
		return core.Err(status, code, detail)
	}
}

// New construye el servidor con las rutas de sistema. Los módulos se agregan después
// con sus propias funciones Register.
func New(s *core.Settings, log *slog.Logger) *Host {
	r := chi.NewRouter()
	h := &Host{S: s, Lim: core.NewLimiter(), Jobs: core.NewJobRegistry(s.JobsDir()), Router: r, Log: log}

	r.Use(h.recoverer, h.timing, h.cors)
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		core.WriteError(w, http.StatusNotFound, "not_found", "Not Found", nil)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		core.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
	})

	cfg := huma.DefaultConfig(s.AppName, core.Version)
	cfg.CreateHooks = nil // sin el campo "$schema" ni el header Link: el contrato no cambia
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		securityScheme: {Type: "apiKey", In: "header", Name: core.APIKeyHeader},
	}
	if s.IsProd() {
		cfg.OpenAPIPath, cfg.DocsPath, cfg.SchemasPath = "", "", ""
	} else {
		cfg.OpenAPIPath, cfg.DocsPath, cfg.SchemasPath = "/openapi", "/docs", ""
	}
	// El esquema del envelope de error se llama «ErrorResponse», no «APIError».
	cfg.Components.Schemas = huma.NewMapRegistry("#/components/schemas/", func(t reflect.Type, hint string) string {
		if t == reflect.TypeFor[core.APIError]() {
			return "ErrorResponse"
		}
		return huma.DefaultSchemaNamer(t, hint)
	})
	h.API = humachi.New(r, cfg)

	registerSystem(h)
	return h
}

// Handler es el http.Handler listo para http.Server.
func (h *Host) Handler() http.Handler { return h.Router }

// Public registra un endpoint con solo rate limit.
// trimDocs quita del OpenAPI el 500 genérico que huma agrega a toda operación con errores:
// no forma parte del contrato y `gen:api` generaría tipos de más.
func (h *Host) trimDocs(op huma.Operation) {
	item := h.API.OpenAPI().Paths[op.Path]
	if item == nil {
		return
	}
	for _, o := range []*huma.Operation{item.Get, item.Post, item.Put, item.Delete, item.Patch} {
		if o != nil && o.OperationID == op.OperationID {
			delete(o.Responses, "500")
		}
	}
}

func Public[I, O any](h *Host, op huma.Operation, handler func(ctx context.Context, in *I) (*O, error)) {
	op.Middlewares = huma.Middlewares{h.rateLimit()}
	huma.Register(h.API, op, handler)
	h.trimDocs(op)
}

// Protected registra un endpoint con rate limit y luego API key (el límite va antes de la
// autenticación: también frena intentos con claves inválidas).
func Protected[I, O any](h *Host, op huma.Operation, handler func(ctx context.Context, in *I) (*O, error)) {
	op.Middlewares = huma.Middlewares{h.rateLimit(), h.auth()}
	op.Security = []map[string][]string{{securityScheme: {}}}
	op.Errors = append(op.Errors, http.StatusUnauthorized, http.StatusServiceUnavailable)
	huma.Register(h.API, op, handler)
	h.trimDocs(op)
}

// ProtectedWith es Protected con middlewares adicionales por endpoint (p. ej. h.Extra("3/minute", "run")),
// que corren después del límite global y de la autenticación.
func ProtectedWith[I, O any](h *Host, op huma.Operation, handler func(ctx context.Context, in *I) (*O, error),
	extra ...func(huma.Context, func(huma.Context))) {
	op.Middlewares = append(huma.Middlewares{h.rateLimit(), h.auth()}, extra...)
	op.Security = []map[string][]string{{securityScheme: {}}}
	op.Errors = append(op.Errors, http.StatusUnauthorized, http.StatusServiceUnavailable)
	huma.Register(h.API, op, handler)
	h.trimDocs(op)
}

// Concurrent limita las ejecuciones simultáneas de un endpoint costoso (si está lleno responde 503 «busy» sin esperar.
func (h *Host) Concurrent(n int) func(huma.Context, func(huma.Context)) {
	gate := core.NewGate(n)
	return func(ctx huma.Context, next func(huma.Context)) {
		release, err := gate.Acquire()
		if err != nil {
			h.reject(ctx, err, nil)
			return
		}
		defer release()
		next(ctx)
	}
}

// Extra agrega un límite estricto por endpoint (p. ej. "3/minute" en un POST /run).
func (h *Host) Extra(limit, scope string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		req := &http.Request{Header: http.Header{}, RemoteAddr: ctx.RemoteAddr()}
		if v := ctx.Header("X-Forwarded-For"); v != "" {
			req.Header.Set("X-Forwarded-For", v)
		}
		if e := h.Lim.Check(h.S, limit, scope, req); e != nil {
			h.reject(ctx, e, nil)
			return
		}
		next(ctx)
	}
}

func (h *Host) rateLimit() func(huma.Context, func(huma.Context)) {
	return h.Extra(h.S.RateLimit, "default")
}

func (h *Host) auth() func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if !h.S.KeyValid(ctx.Header(core.APIKeyHeader)) {
			h.reject(ctx, core.Err(http.StatusUnauthorized, "unauthorized", "API key inválida o ausente."),
				map[string]string{"WWW-Authenticate": "ApiKey"})
			return
		}
		next(ctx)
	}
}

func (h *Host) reject(ctx huma.Context, e *core.APIError, headers map[string]string) {
	ctx.SetHeader("Content-Type", "application/json")
	for k, v := range headers {
		ctx.SetHeader(k, v)
	}
	ctx.SetStatus(e.Status)
	_ = json.NewEncoder(ctx.BodyWriter()).Encode(e)
}

// --- middlewares de chi ---------------------------------------------------------

type statusWriter struct {
	http.ResponseWriter
	start  time.Time
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wrote {
		w.wrote, w.status = true, code
		w.Header().Set("X-Process-Time-ms", fmt.Sprintf("%.1f", float64(time.Since(w.start).Microseconds())/1000))
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (h *Host) timing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, start: time.Now()}
		next.ServeHTTP(sw, r)
		h.Log.Debug("request", "method", r.Method, "path", r.URL.Path, "status", sw.status,
			"ms", time.Since(sw.start).Milliseconds())
	})
}

func (h *Host) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler { //nolint:errorlint // el valor de panic no es un error envuelto
					panic(rec)
				}
				h.Log.Error("error no controlado", "method", r.Method, "path", r.URL.Path,
					"panic", fmt.Sprint(rec), "stack", string(debug.Stack()))
				detail := "Error interno del servidor."
				if !h.S.IsProd() {
					detail = fmt.Sprintf("panic: %v", rec)
				}
				core.WriteError(w, http.StatusInternalServerError, "internal_error", detail, nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// cors replica CORSMiddleware de Starlette: orígenes de ALLOWED_ORIGINS, GET y POST,
// headers X-API-Key y Content-Type, sin credenciales.
func (h *Host) cors(next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range h.S.Origins {
		allowed[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			if !allowed[origin] {
				http.Error(w, "Disallowed CORS origin", http.StatusBadRequest)
				return
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
			w.Header().Set("Access-Control-Allow-Headers", core.APIKeyHeader+", Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
