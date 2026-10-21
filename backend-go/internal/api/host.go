// Package api arma el servidor HTTP: huma sobre chi, con el contrato público de la API
// (rutas, envelope de error {detail, code}, CORS, rate limit y API key).
package api

import (
	"context"
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

// Host reúne lo que comparten todos los routers: configuración y registro de jobs.
// Cada módulo registra sus endpoints con Public.
type Host struct {
	S      *core.Settings
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
	h := &Host{S: s, Jobs: core.NewJobRegistry(s.JobsDir()), Router: r, Log: log}

	r.Use(h.recoverer, h.timing)
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		core.WriteError(w, http.StatusNotFound, "not_found", "Not Found", nil)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		core.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed", nil)
	})

	cfg := huma.DefaultConfig(s.AppName, core.Version)
	cfg.CreateHooks = nil // sin el campo "$schema" ni el header Link: el contrato no cambia
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

// Public registra un endpoint GET (la API es de solo lectura y de uso local: sin autenticación).
func Public[I, O any](h *Host, op huma.Operation, handler func(ctx context.Context, in *I) (*O, error)) {
	huma.Register(h.API, op, handler)
	h.trimDocs(op)
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
