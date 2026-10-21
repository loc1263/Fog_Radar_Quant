// Command fogradar es el binario del backend Go. Subcomandos:
//
//	fogradar api        # sirve la API HTTP (Fase 1)
//	fogradar openapi    # imprime el OpenAPI generado
//	fogradar export     # instantánea JSON de la API para el sitio estático (--out, --modulo)
//	fogradar job ...    # pipelines batch: un módulo, --todos o --programa (ver `fogradar job --list`)
//

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fogradar/internal/api"
	"fogradar/internal/core"
	"fogradar/internal/modules/cl"
	"fogradar/internal/modules/etf"
	"fogradar/internal/modules/tend"
	"fogradar/internal/modules/us"
	"fogradar/internal/transversal"
)

func main() {
	cmd := "api"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	if err := run(cmd); err != nil {
		fmt.Fprintln(os.Stderr, "fogradar:", err)
		if errors.Is(err, errUso) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

var errUso = errors.New("uso: fogradar [api|openapi|export|job]")

func newHost() (*api.Host, error) {
	s, err := core.LoadSettings()
	if err != nil {
		return nil, err
	}
	log, _ := newLogger(s, os.Stdout, "api") // vive lo que el proceso
	h := api.New(s, log)
	cl.Register(h)
	tend.Register(h)
	us.Register(h)
	etf.Register(h)
	transversal.Register(h)
	return h, nil
}

func run(cmd string) error {
	switch cmd {
	case "api":
		return serve()
	case "job":
		os.Exit(runJob(os.Args[2:], os.Stdout, os.Stderr))
		return nil
	case "export":
		os.Exit(runExport(os.Args[2:], os.Stdout, os.Stderr))
		return nil
	case "openapi":
		h, err := newHost()
		if err != nil {
			return err
		}
		b, err := h.API.OpenAPI().MarshalJSON()
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	default:
		return fmt.Errorf("%w (recibido %q)", errUso, cmd)
	}
}

func serve() error {
	h, err := newHost()
	if err != nil {
		return err
	}
	s := h.S
	if err := os.MkdirAll(s.DataDir, 0o755); err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              s.Addr,
		Handler:           h.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	h.Log.Info("arrancando", "app", s.AppName, "version", core.Version, "env", s.Env,
		"data", s.DataDir, "addr", s.Addr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		sd, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(sd)
	}
}
