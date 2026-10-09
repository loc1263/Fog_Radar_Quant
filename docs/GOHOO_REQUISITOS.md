# gohoo: qué le falta para cubrir el 100 % de Fog Radar Quant

Documento de requisitos para la siguiente versión de `github.com/loc1263/gohoo` (hoy **v0.2.1**). `gohoo` es una librería propia
y la usan otros proyectos, así que **todo cambio propuesto es aditivo**: ninguna firma ni comportamiento por defecto cambia.
Contexto de la migración: `roadmap_go.md` y `docs/PARIDAD_GO.md`.

## 1. Resumen

Con v0.2.1 el backend Go ya funciona (Chile, EEUU, ETF y Tendencia), pero con **una capa de parches propia** en
`backend-go/internal/yahoo` y **una funcionalidad que no se pudo portar** (el screener de universo de Tendencia).
Lo que falta, en orden de prioridad:

| # | Falta en gohoo | Hoy en el proyecto | Prioridad |
|---|---|---|---|
| 1 | **Screener** (`EquityQuery` + POST autenticado) | No existe en Go: el descubrimiento de universo sigue en Python | **P0** (única brecha funcional) |
| 2 | **Petición autenticada genérica** (`GetJSON`/`PostJSON` con cookie + crumb) | Imposible llamar endpoints nuevos sin copiar la lógica de sesión | **P0** (lo necesita el #1) |
| 3 | **Cotizaciones en lote** (`/v7/finance/quote`) | 1 `quoteSummary` por ticker para el market cap (~500 requests en EEUU) | P1 (mayor ahorro de tiempo) |
| 4 | **`Info()`**: aplanado tipo `Ticker.info` de yfinance | `FlattenInfo` + tabla de prioridad por módulo + ×100 de `dividendYield` | P1 |
| 5 | **Ganchos de tráfico** (limitador, eventos por request) | Envoltorio `Traffic` que cuenta 429 y espacia requests desde fuera | P1 |
| 6 | **Política de reintento por llamada** (modo interactivo) | `Interactive(ctx)` propio que desactiva el backoff largo | P2 |
| 7 | **Barras «tipo yfinance»** (`auto_adjust`, fecha local) | `AutoAdjust` propio | P2 |
| 8 | **Fundamentals-timeseries tipado** | `TimeseriesPoints`, `TrailingPEG` propios | P2 |
| 9 | Calidad: fixtures grabados, pruebas de contrato, rangos válidos, docs | — | P2–P3 |

## 2. Qué usa hoy el proyecto de gohoo

| Función de gohoo | Uso |
|---|---|
| `ChartRange` / `ChartParams` / `ChartResult` | Históricos (semanal 5y de Tendencia, diario 1y–5y de EEUU, ETF y Chile; FX; VIX) |
| `QuoteSummaryContext` | `info` de Chile y ETF (módulos `price`, `summaryDetail`, `defaultKeyStatistics`, `financialData`, `assetProfile`, `quoteType`, `fundProfile`); market cap de EEUU (`price`) |
| `FundamentalsTimeseriesContext` | Fundamentales trimestrales de Chile (`quarterly*`) y `trailingPegRatio` |
| `RawValue`, `AsFloat`, `GetPath` | Lectura del árbol JSON de `quoteSummary` |
| `StatusError`, `ErrRateLimit`, `ErrNotFound`, `IsRateLimit` | Clasificación de errores en el reintento propio |
| `RetryPolicy` | Se usa el de por defecto; el control fino (espaciado global, backoff largo en batch) lo hace el proyecto por fuera |

Sin problemas hallados: cierres idénticos a yfinance en 42 tickers (EEUU, `.SN`, ETF, Asia, FX, índices, BTC), 0 respuestas 429/401/403
en ~1 000 requests de pipeline con espaciado, ~0,25 s por `quoteSummary` (vs 1,35 s de `Ticker.info`).

## 3. Propuestas (todas aditivas)

> Las formas de los endpoints de Yahoo descritas abajo (cuerpo del screener, campos de `quote`) vienen de cómo los usa `yfinance`
> y **hay que verificarlas contra Yahoo antes de implementar**; no se probaron desde este entorno.

### 3.1 Petición autenticada genérica — P0

Hoy la sesión (cookie jar, crumb, renovación ante 401, reintentos) es privada: para un endpoint nuevo hay que copiarla.

```go
// GET con cookie + crumb (el crumb se agrega como query "crumb"), reintentos de la RetryPolicy y renovación en 401.
func (c *Client) GetJSON(ctx context.Context, path string, q url.Values) ([]byte, error)
// POST con cuerpo JSON y la misma sesión.
func (c *Client) PostJSON(ctx context.Context, path string, q url.Values, body any) ([]byte, error)
```
`quoteSummary`, `fundamentals` y `chart` pasarían a usarlas internamente (sin cambiar su API pública). Criterio: los tests actuales
con `httptest` siguen en verde y `apidiff` no reporta cambios incompatibles.

### 3.2 Screener — P0

Lo usa `tendencia/universe/discovery.py` (`yf.EquityQuery` + `yf.screen`): por región (`cn hk jp kr tw in us`), ordenado por
`intradaymarketcap` descendente, hasta 150 candidatos; en modo mega-cap además filtra `marketCap ≥ 200 000 M USD`.

```go
type Query struct{ /* árbol de operadores */ }
func Eq(field string, v any) Query          // eq
func Gt(field string, v float64) Query      // gt (también Lt, Gte, Lte)
func Btwn(field string, lo, hi float64) Query
func In(field string, vs ...any) Query      // is-in
func And(qs ...Query) Query
func Or(qs ...Query) Query

type ScreenParams struct {
    Query     Query
    QuoteType string // "EQUITY" (defecto), "ETF", "MUTUALFUND"
    Offset    int
    Size      int    // Yahoo limita (≈250 por página)
    SortField string // p. ej. "intradaymarketcap"
    SortAsc   bool
}
type ScreenQuote struct {
    Symbol, ShortName, LongName, DisplayName string
    QuoteType, Currency, Exchange, FullExchangeName, Region string
    MarketCap float64          // ya sin el envoltorio {raw, fmt}
    Raw       map[string]any   // el resto de campos tal cual
}
type ScreenResult struct{ Total int; Quotes []ScreenQuote }

func (c *Client) Screen(ctx context.Context, p ScreenParams) (*ScreenResult, error)
func (c *Client) ScreenPredefined(ctx context.Context, id string, offset, size int) (*ScreenResult, error) // opcional
```
- Endpoint de referencia: `POST /v1/finance/screener` con cuerpo `{offset, size, sortField, sortType, quoteType, query, userId, userIdType}`
  más `crumb`, `lang`, `region` en la query (verificar).
- Errores tipados: respuesta sin `quotes` → `ErrEmptyScreen` (hoy el proyecto trata «sin quotes» como cambio de API o región vacía).
- Solo requiere la sesión del #3.1; sin dependencias nuevas.
- Aprovechamiento en el proyecto: además de Tendencia, el screener de ETF (`quoteType: "ETF"`) podría reemplazar parte de las listas CSV.

### 3.3 Cotizaciones en lote — P1

`fast_info`/market cap de EEUU y el spot de FX hoy cuestan **1 request por símbolo** (`quoteSummary` o `chart` de 5 días). Yahoo
tiene un endpoint de cotizaciones por lote (`/v7/finance/quote?symbols=A,B,C…`, cientos por llamada).

```go
type Quote struct {
    Symbol, Currency, QuoteType, ShortName, LongName, Exchange string
    Price, MarketCap, PreviousClose float64
    Raw map[string]any
}
// Parte los símbolos en lotes (tamaño configurable, ≈ 100) y devuelve los que Yahoo conoce; los desconocidos se omiten.
func (c *Client) Quote(ctx context.Context, symbols []string) ([]Quote, error)
```
Efecto estimado: el paso «market cap» de EEUU (≈ 500 requests, ~117 s de espaciado + latencia) baja a ~5 requests; el spot de FX
del conversor `fx.Rates` también. Es la mejora de **mayor ahorro de tiempo** de toda la lista.

### 3.4 `Info()` tipo yfinance — P1

El proyecto mantiene en `internal/yahoo/info.go` el mapeo de `Ticker.info`. Reglas que conviene que vivan en la librería (con tests por campo):
- Prioridad de módulos al aplanar (la primera ocurrencia gana): `summaryDetail`, `price`, `defaultKeyStatistics`, `financialData`, `assetProfile`, `quoteType`.
  `forwardPE` debe salir de `summaryDetail`: el de `defaultKeyStatistics` viene corrupto en algunas acciones chilenas (11 454 en vez de 10,8).
- `dividendYield`: yfinance lo entrega en **porcentaje** y Yahoo en fracción → **opción** (`InfoPercentYield(true)`), nunca por defecto; en ETFs
  `summaryDetail` no lo trae y se usa `yield`.
- `trailingPegRatio` viene de fundamentals-timeseries, no de `quoteSummary` (yfinance lo agrega a `info`).

```go
type InfoOption func(*infoConfig)
func InfoModules(m ...string) InfoOption
func InfoPercentYield(on bool) InfoOption
func InfoWithTrailingPEG(on bool) InfoOption
func FlattenInfo(qs map[string]any, opts ...InfoOption) map[string]any
func (c *Client) Info(ctx context.Context, ticker string, opts ...InfoOption) (map[string]any, error)
```

### 3.5 Ganchos de tráfico — P1

Tendencia, ETF y EEUU comparten un límite global (espaciado mínimo entre *inicios* de request, contadores de 429). Hoy se logra envolviendo
cada llamada del cliente desde fuera, lo que obliga a duplicar las firmas (`Source`, `Client` de `internal/yahoo`).

```go
type Limiter interface{ Wait(ctx context.Context) error }
func WithLimiter(l Limiter) Option                 // se invoca antes de CADA intento HTTP (incluidos los reintentos)

type RequestEvent struct {
    Endpoint string        // "chart", "quoteSummary", "fundamentals", "screener", "quote"
    Symbol   string
    Attempt  int
    Status   int           // 0 si falló la red
    Duration time.Duration
    Err      error
}
func WithRequestHook(fn func(RequestEvent)) Option
```
Con esto el proyecto puede borrar su `Traffic.Do`/`Client` envolvente y obtener estadísticas exactas por intento (hoy sólo ve el resultado final).

### 3.6 Política de reintento por llamada — P2

Dentro de un request web (evaluación en vivo de un ticker) el proyecto no puede esperar backoffs largos; en batch sí. Hoy lo resuelve con un
`context` propio. Propuesta:

```go
func WithRetryPolicyContext(ctx context.Context, p RetryPolicy) context.Context // tiene prioridad sobre la del cliente
```

### 3.7 Barras «tipo yfinance» — P2

El proyecto necesita (a) `auto_adjust` (Close = AdjClose y escalar O/H/L con el mismo factor) y (b) la hora de cada vela diaria/semanal llevada
a medianoche local (yfinance indexa por fecha; las comparaciones «último dato en o antes de…» dependen de eso).

```go
func (b Bar) Date() time.Time                       // medianoche en b.Time.Location()
func (r *ChartResult) AutoAdjusted() []Bar          // copia con auto_adjust y Time normalizado a Date()
```
No cambia `Bars` ni `Monthly()`.

### 3.8 Fundamentals-timeseries tipado — P2

```go
type Point struct{ Date string; Value float64 }
// Observaciones de un tipo (p. ej. "quarterlyTotalRevenue"), la más reciente primero, sin faltantes.
func TimeseriesPoints(data map[string]any, typ string) []Point
func (c *Client) TrailingPEG(ctx context.Context, ticker string) (v float64, ok bool, err error)
```

### 3.9 Calidad y mantenimiento — P2/P3

- **Fixtures grabados** de respuestas reales (`chart`, `quoteSummary` de EEUU/Chile/ETF, `fundamentals-timeseries`, screener, quote) en `testdata/` para
  pruebas sin red; las pruebas contra Yahoo real, solo con `-tags live`.
- **Rangos y intervalos válidos**: validar `Range`/`Interval` con error claro (el proyecto usa `14mo`, `5y`, `1wk`) y documentar la tabla de combinaciones permitidas.
- **Documentación** (`API.md`, `ENDPOINTS.md`, `README.es.md`): ejemplos de screener, quote en lote e `Info`.
- **CI**: mantener `apidiff` contra la versión anterior (ya existe) y agregar `golangci-lint`, `go test -race` y una matriz Linux/Windows.

## 4. Reglas de compatibilidad para los otros proyectos

1. **Sólo se agregan** funciones, tipos y opciones. Ninguna firma exportada cambia; `apidiff` contra v0.2.1 debe dar «sin cambios incompatibles».
2. **Mismos valores por defecto**: `NewClient()` se comporta igual (bootstrap inmediato, `DefaultRetryPolicy`, UA por defecto). Todo lo nuevo se activa con opciones.
3. **Sin dependencias nuevas** (el screener y el quote en lote son HTTP + JSON); `go.mod` conserva la versión mínima de Go actual.
4. **Versión**: `v0.3.0` (minor) con entrada en `CHANGELOG.md` por cambio. Las correcciones de comportamiento (como la de `adjclose` nulo de v0.2.1) siguen en
   patch y se anuncian como `Fixed`.
5. **Mismo nombre de errores**: lo nuevo reutiliza `StatusError`, `ErrRateLimit`, `ErrNotFound`, `ErrUnauthorized`; sólo se suman `ErrEmptyScreen` y similares.
6. Los campos nuevos de structs existentes (si hubiera) se agregan al final; los structs nuevos no exponen campos sin documentar.

## 5. Orden de trabajo sugerido

1. **#3.1 `GetJSON`/`PostJSON`** (refactor interno + tests existentes en verde).
2. **#3.2 `Screen`** con fixtures grabados → habilita el descubrimiento de universo en Go.
3. **#3.3 `Quote`** → reemplaza el market cap por ticker y el spot de FX.
4. **#3.5 Ganchos** y **#3.4 `Info`** → el proyecto elimina `internal/yahoo/{client,traffic,info,fundamentals}.go` en lo que duplica.
5. #3.6, #3.7, #3.8 y #3.9.
6. Publicar `v0.3.0` y actualizar `backend-go/go.mod`.

## 6. Qué cambia en este proyecto cuando gohoo lo tenga

- `backend-go/internal/modules/tend/pipeline.go`: reemplazar `resolveUniverse` por el descubrimiento real (port de `discovery.py` sobre `Screen`) y cerrar el
  único pendiente funcional de la Fase 7.
- `backend-go/internal/modules/us/scanner.go` y `internal/fx/fx.go`: usar `Quote` por lotes.
- `backend-go/internal/yahoo/`: reducir a la política del proyecto (límite global y reglas de `Interactive`) sobre `WithLimiter`/`WithRequestHook`; mover a gohoo el aplanado de `info`,
  `AutoAdjust` y `TimeseriesPoints`.
- Actualizar `docs/PARIDAD_GO.md` (diferencia #9, screener) y `roadmap_go.md` §13.

## 7. Criterios de aceptación

- Un caso de prueba por función nueva con fixtures grabados y otro `-tags live` opcional.
- `go vet`, `golangci-lint` y `go test -race ./...` en verde; `apidiff` sin cambios incompatibles.
- Los proyectos que hoy usan gohoo compilan y pasan sus tests **sin modificar su código** al subir a v0.3.0.
- En este proyecto: el descubrimiento de universo en Go produce la misma lista (regiones, top-N y filtro de sede en EEUU) que Python con los mismos datos de Yahoo, y el paso de
  market cap de EEUU baja de ~500 requests a ≤ 10.
