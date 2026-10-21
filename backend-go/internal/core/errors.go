package core

import (
	"encoding/json"
	"net/http"
)

// APIError es un error de dominio con código estable. Su JSON es el envelope
// uniforme del backend: {"detail": "...", "code": "..."}.
type APIError struct {
	Status int    `json:"-"`
	Detail string `json:"detail" doc:"Mensaje legible"`
	Code   string `json:"code" doc:"Código estable (no_data, busy, not_found…)"`
}

func (e *APIError) Error() string  { return e.Detail }
func (e *APIError) GetStatus() int { return e.Status }

// Err construye un APIError.
func Err(status int, code, detail string) *APIError {
	return &APIError{Status: status, Code: code, Detail: detail}
}

// NoData es el 503 de "todavía no hay corrida".
func NoData(detail string) *APIError {
	if detail == "" {
		detail = "Todavía no hay datos para este módulo."
	}
	return Err(http.StatusServiceUnavailable, "no_data", detail)
}

var httpCodes = map[int]string{
	400: "bad_request", 401: "unauthorized", 403: "forbidden", 404: "not_found",
	405: "method_not_allowed", 409: "conflict", 429: "rate_limited",
}

// CodeForStatus es el código del envelope para un status HTTP genérico.
func CodeForStatus(status int) string {
	if c, ok := httpCodes[status]; ok {
		return c
	}
	return "http_error"
}

// WriteError escribe el envelope en un ResponseWriter (middlewares y handlers de chi).
func WriteError(w http.ResponseWriter, status int, code, detail string, headers map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	for k, v := range headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(&APIError{Detail: detail, Code: code})
}
