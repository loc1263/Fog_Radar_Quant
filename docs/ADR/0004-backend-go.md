# ADR 0004 — Backend en Go

**Estado:** aceptado.

## Contexto
Los pipelines dependen de Yahoo (espera de red, no de CPU), la memoria por proceso importa y el despliegue debe ser simple.

## Decisión
El backend es un binario Go (`backend-go/`: huma sobre chi, sqlite sin CGO, `gohoo` para Yahoo, gonum). Un solo ejecutable
sirve la API (`fogradar api`) y corre los pipelines batch (`fogradar job`), con el contrato público de `/api`: mismas rutas,
mismo OpenAPI, mismo envelope de error, mismos archivos en `data/` y mismo esquema de `history.db`.

## Consecuencias
- Binario estático (~25 MB, imagen distroless); pipelines rápidos (Chile ≈ 40 s, EEUU ≈ 4 min, ETF ≈ 7–13 min).
- Locks entre procesos (`.pipelines.lock`, `current/.datos.lock`, `<archivo>.jsonl.lock`) y migraciones de `history.db`
  (nunca se edita una publicada).
- Gestor y calidad: `go`, `gofmt`, `go vet`, `golangci-lint`, `go test`; CLI `fogradar job`.
- Los tipos del frontend se generan desde el OpenAPI del propio backend (`npm run gen:api`).
