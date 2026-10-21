package etf

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"fogradar/internal/api"
	"fogradar/internal/core"
)

type pdfCache struct {
	mu    sync.Mutex
	clave string
	pdf   []byte
	fecha string
}

var cachePDF pdfCache

// firmaDatos identifica el estado de los CSV (nombre, tamaño, mtime) de current/ y hist/ más el día de emisión: el PDF
// solo cambia cuando cambian los archivos.
func firmaDatos(s *core.Settings, hoy time.Time) string {
	var b strings.Builder
	b.WriteString("hoy/" + hoy.Format("2006-01-02") + "\n")
	for _, dir := range []string{CurrentDir(s), HistDir(s)} {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		sort.Slice(ents, func(i, j int) bool { return ents[i].Name() < ents[j].Name() })
		for _, e := range ents {
			if info, err := e.Info(); err == nil && !e.IsDir() {
				fmt.Fprintf(&b, "%s/%s:%d:%d\n", filepath.Base(dir), e.Name(), info.Size(), info.ModTime().UnixNano())
			}
		}
	}
	return b.String()
}

// registerPDF agrega GET /api/etf/pdf (informe ejecutivo de la corrida vigente).
func registerPDF(h *api.Host) {
	op := huma.Operation{OperationID: "etf_pdf", Method: http.MethodGet, Path: "/api/etf/pdf",
		Summary: "Informe ejecutivo en PDF de la corrida vigente", Tags: []string{tag},
		Responses: map[string]*huma.Response{"200": {Description: "PDF", Content: map[string]*huma.MediaType{"application/pdf": {}}}}}
	api.Public(h, op, func(context.Context, *struct{}) (*huma.StreamResponse, error) {
		unlock, e := BloqueoDatos(h.S, 0)
		if e != nil {
			return nil, e
		}
		defer unlock()
		clave := firmaDatos(h.S, time.Now().In(h.S.Loc))
		cachePDF.mu.Lock()
		defer cachePDF.mu.Unlock()
		if cachePDF.clave != clave {
			pdfBytes, fecha, err := GenerarPDF(CurrentDir(h.S), HistDir(h.S))
			if errors.Is(err, ErrSinCorrida) {
				return nil, core.NoData(err.Error())
			}
			if err != nil {
				return nil, core.Err(http.StatusInternalServerError, "pipeline_error", "Error generando el informe PDF; revisa el log del servidor.")
			}
			cachePDF.clave, cachePDF.pdf, cachePDF.fecha = clave, pdfBytes, fecha
		}
		body, nombre := cachePDF.pdf, "etf_informe_"+strings.ReplaceAll(cachePDF.fecha, "/", "")+".pdf"
		return &huma.StreamResponse{Body: func(ctx huma.Context) {
			ctx.SetHeader("Content-Type", "application/pdf")
			ctx.SetHeader("Content-Disposition", `attachment; filename="`+nombre+`"`)
			_, _ = ctx.BodyWriter().Write(body)
		}}, nil
	})
}
