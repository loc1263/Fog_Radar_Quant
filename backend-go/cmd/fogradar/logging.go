package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"fogradar/internal/core"
)

const (
	logFileName = "fogradar.log"
	logMaxBytes = 20 << 20 // al superarlo, el log anterior pasa a fogradar.log.1
)

// newLogger arma el logger del proceso: consola (nivel LOG_LEVEL) + el log único
// data/_jobs/logs/fogradar.log (siempre ≥ INFO, con proc y pid), compartido por la API y los jobs.
// Si el archivo no se puede abrir, sigue solo con la consola. close cierra el archivo.
func newLogger(s *core.Settings, console io.Writer, proc string) (log *slog.Logger, close func()) {
	var lvl slog.Level
	if lvl.UnmarshalText([]byte(s.LogLevel)) != nil {
		lvl = slog.LevelInfo
	}
	con := slog.NewTextHandler(console, &slog.HandlerOptions{Level: lvl})
	f, err := abrirLogUnico(s)
	if err != nil {
		l := slog.New(con)
		l.Warn("no se pudo abrir el log único; solo consola", "error", err)
		return l, func() {}
	}
	fl := min(lvl, slog.LevelInfo)
	fh := slog.NewTextHandler(f, &slog.HandlerOptions{Level: fl}).
		WithAttrs([]slog.Attr{slog.String("proc", proc), slog.Int("pid", os.Getpid())})
	return slog.New(teeHandler{con, fh}), func() { _ = f.Close() }
}

func abrirLogUnico(s *core.Settings) (*os.File, error) {
	dir := filepath.Join(s.JobsDir(), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, logFileName)
	if st, err := os.Stat(p); err == nil && st.Size() > logMaxBytes {
		_ = os.Rename(p, p+".1") // si falla (otro proceso lo tiene abierto en Windows) se sigue anexando
	}
	return os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

// teeHandler envía cada registro a todos los handlers que lo acepten según su nivel.
type teeHandler []slog.Handler

func (t teeHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range t {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (t teeHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range t {
		if h.Enabled(ctx, r.Level) {
			errs = append(errs, h.Handle(ctx, r.Clone()))
		}
	}
	return errors.Join(errs...)
}

func (t teeHandler) WithAttrs(a []slog.Attr) slog.Handler {
	out := make(teeHandler, len(t))
	for i, h := range t {
		out[i] = h.WithAttrs(a)
	}
	return out
}

func (t teeHandler) WithGroup(n string) slog.Handler {
	out := make(teeHandler, len(t))
	for i, h := range t {
		out[i] = h.WithGroup(n)
	}
	return out
}
