# syntax=docker/dockerfile:1
# Imagen del backend Go: binario estático (sqlite sin CGO, tzdata embebida) sobre distroless.
# Contexto de build: backend-go/   →   docker build -t fogradar-api backend-go
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=0.1.0
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X fogradar/internal/core.Version=${VERSION}" \
    -o /out/fogradar ./cmd/fogradar

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/fogradar /fogradar
# Los datos viven fuera de la imagen (volumen compartido con los pipelines).
ENV DATA_DIR=/data ADDR=:8000
EXPOSE 8000
USER nonroot:nonroot
ENTRYPOINT ["/fogradar"]
CMD ["api"]
