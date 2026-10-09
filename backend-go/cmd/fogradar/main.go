// Command fogradar es el binario del backend Go. Subcomandos:
//
//	fogradar api        # sirve la API HTTP (Fase 1)
//	fogradar openapi    # imprime el OpenAPI generado
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
	"strings"
	"syscall"
	"time"

	"fogradar/internal/api"
	"fogradar/internal/core"
	"fogradar/internal/jobs"
	"fogradar/internal/modules/cl"
	"fogradar/internal/modules/etf"
	"fogradar/internal/modules/tend"
	"fogradar/internal/modules/us"
	"fogradar/internal/transversal"
	"fogradar/internal/yahoo"
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

var errUso = errors.New("uso: fogradar [api|openapi|job]")

func newHost() (*api.Host, error) {
	s, err := core.LoadSettings()
	if err != nil {
		return nil, err
	}
	log, _ := newLogger(s, os.Stdout, "api") // vive lo que el proceso
	h := api.New(s, log)
	// Cliente de Yahoo del proceso de la API: solo lo usa la evaluación en vivo de un ticker (US).
	tr := yahoo.NewTraffic(yahoo.PolicyFromSettings(s))
	tr.SetLogger(func(f string, a ...any) { log.Warn(fmt.Sprintf(f, a...)) })
	client := yahoo.NewClient(tr)
	cl.Register(h)
	tend.Register(h)
	us.Register(h, client)
	// POST /api/etf/run re-analiza sin Yahoo; el entorno de jobs se arma al usarlo.
	etf.Register(h, func() *jobs.Env { return &jobs.Env{S: s, Traffic: tr, Yahoo: client, Log: log} })
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
	if s.APIKey == "" {
		h.Log.Warn("API_KEY no configurada: la API acepta requests sin autenticación (solo dev/test).")
	}
	srv := &http.Server{
		Addr:              s.Addr,
		Handler:           h.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	h.Log.Info("arrancando", "app", s.AppName, "version", core.Version, "env", s.Env,
		"data", s.DataDir, "addr", s.Addr, "origins", strings.Join(s.Origins, ","))

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
