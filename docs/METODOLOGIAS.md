# Metodologías

Este documento describe **cómo calcula hoy cada módulo** de Fog Radar Quant. Se armó a partir
del código del backend (`backend/src/fog_radar/`), no de los README de los proyectos
originales. Si el código cambia, este documento debe cambiar con él. Las rutas de archivo que
se citan son relativas a `backend/src/fog_radar/`.

| Módulo | Pregunta que responde | Unidad de análisis |
|---|---|---|
| Acciones Chile (IPSA) | ¿Qué acciones del IPSA se ven mejor en un score multifactor (valor, calidad, crecimiento, salud, dividendo, momentum)? | 30 acciones `.SN` |
| Acciones EEUU | ¿Qué mega-caps de EEUU tienen mejor momentum 12-1, qué tan "sanas" están y cómo combinarlas en un portafolio? | Acciones con market cap ≥ USD 200 mil millones |
| ETFs | ¿Qué ETFs listados en EEUU tienen mejor relación retorno/riesgo, más momentum y cómo rotan semana a semana? | ~1.225 ETFs de una lista curada |
| Tendencia (Asia + EEUU) | ¿Qué mega-caps de Asia y EEUU entran a un portafolio Markowitz, qué tendencia tienen y qué rango de precio es plausible a 6 meses? | Universo dinámico (top 20 Asia + mega-caps EEUU) |

## Fuentes de datos

- **Yahoo Finance** (librería `gohoo`), para precios, fundamentales, FX y el screener de
  capitalización. Todas las llamadas del proceso pasan por **un solo cliente** (`internal/yahoo`, creado por
  `fogradar api` y `fogradar job`):
  - espaciado mínimo global entre requests de **0,35 s + jitter U(0; 0,25) s**, compartido
    entre hilos;
  - ante HTTP 429, hasta **3 reintentos** con backoff `min(5 · 2^intento, 60) + U(0, 1)` s;
    hasta 2 reintentos por errores de red;
  - dentro de un request web (`yahoo.interactivo()`) un 429 se devuelve de inmediato, sin
    backoff;
  - caché de históricos en disco (la usa Tendencia, con frescura de 20 h).
  Cada módulo conserva además sus propias pausas anti-bloqueo (se detallan abajo).
- **FRED** (Federal Reserve Economic Data), solo en Tendencia y solo si se corre con
  `--macro` y hay `FRED_API_KEY` (`internal/modules/tend/macro.go`): curva de tasas (DGS3MO,
  DGS2, DGS10, DGS30, T10Y2Y, T10Y3M), spreads de crédito ICE BofA (BAMLH0A0HYM2,
  BAMLC0A0CM, BAMLC0A4CBBB), VIXCLS, postura de la Fed (FEDFUNDS, DFEDTARU, DFEDTARL,
  FEDTARMD) y CFNAI como proxy de actividad.
- **Wikipedia** (solo EEUU): componentes del S&P 500 y del Nasdaq-100.

## Calendario de actualización

Definido en `internal/core/schedule.go` (única fuente de verdad; el crontab de producción se genera
desde ahí). Horas de `America/Santiago`.

| Programa | Módulos (en secuencia, nunca en paralelo) | Cuándo |
|---|---|---|
| `semanal` | ETFs | Lunes 06:00 |
| `diario` | Chile → EEUU → Tendencia | Lunes a viernes 18:00 |

La corrida programada de Tendencia **no** incluye hoy `--macro` (pendiente TD-MACRO-1 en el
ROADMAP): usa el último `macro_data_*.csv` disponible, si existe.

---

## 1. Acciones Chile (IPSA)

Archivos: `internal/modules/cl/{extractor,scoring,pipeline}.go`, `internal/fx/fx.go`.

### 1.1 Universo

`modules/cl_stocks/ipsa_tickers.csv`: **30 tickers** del IPSA en formato Yahoo
(`AGUAS-A.SN`, `CHILE.SN`, …), columna `Ticker (Yahoo Finance)`. Es una lista fija
mantenida a mano. Un ticker que falla en la descarga se omite de la corrida y queda
registrado en el resumen del job.

### 1.2 Extracción de indicadores (`extractor.go`)

Por ticker se lee `yf.Ticker(...).info`, los estados trimestrales y un histórico de 2 años
(`history(period="2y", auto_adjust=True)`). Se genera `indicadores_acciones_<timestamp>.csv`
con 61 columnas. Pausa anti-bloqueo: **30 s cada 15 tickers**.

Cálculos propios (no vienen listos de Yahoo para `.SN`):

| Indicador | Cálculo |
|---|---|
| Retorno 5D / 1M / 3M / 6M / 1A | `P_hoy / P_{hoy−n} − 1` con n = 5 / 21 / 63 / 126 / 252 ruedas |
| Retorno YTD | `P_hoy / último cierre del año anterior − 1` |
| Quarterly Revenue Growth (yoy) | `(Rev_t − Rev_{t−4}) / |Rev_{t−4}|`; requiere ≥ 5 trimestres |
| Levered FCF (ttm) | `Operating Cash Flow (ttm) + Σ CapEx de los últimos 4 trimestres` (CapEx viene negativo) |
| Trailing Annual Dividend Yield (%) | fracción de Yahoo × 100 (el forward y el promedio 5 años ya vienen en %) |

**Conversión de moneda (`internal/fx`).** Si `currency` (moneda de cotización, p. ej. CLP)
≠ `financialCurrency` (moneda de los estados, p. ej. USD), los montos contables (Revenue,
EBITDA, book value por acción, caja, deuda, flujos…) se multiplican por la tasa
`financialCurrency → currency`. `FxRates` usa USD como pivote y la convención de Yahoo
`XXX=X` = unidades de XXX por 1 USD (último cierre de 5 días):

- `USD → Y`: `tasa = precio(Y=X)`
- `X → USD`: `tasa = 1 / precio(X=X)`
- `X → Y`: `tasa = (1 / precio(X=X)) · precio(Y=X)`

La tasa (o su fallo, `None`) se cachea por par. Con los montos convertidos se **recalculan**
los múltiplos: `P/S = MarketCap / Revenue`, `P/B = Precio / BookValuePS`,
`EV/Revenue`, `EV/EBITDA`. Si la tasa no se pudo obtener, esos montos quedan vacíos.

### 1.3 Pilares y métricas (`scoring.go`)

20 métricas en 6 pilares. "Invertir" = menor es mejor (se cambia el signo del z-score).
"Solo > 0" = los valores ≤ 0 se descartan porque no tienen lectura económica.

| Pilar | Métrica | Dirección | Filtro |
|---|---|---|---|
| Valor | Trailing P/E | menor es mejor | solo > 0 |
| Valor | Price/Book | menor es mejor | solo > 0 |
| Valor | Price/Sales | menor es mejor | solo > 0 |
| Valor | EV/EBITDA | menor es mejor | solo > 0 |
| Valor | EV/Revenue | menor es mejor | solo > 0 |
| Calidad | Profit Margin | mayor es mejor | — |
| Calidad | Operating Margin (ttm) | mayor es mejor | — |
| Calidad | ROA (ttm) | mayor es mejor | — |
| Calidad | ROE (ttm) | mayor es mejor | — |
| Crecimiento | Quarterly Revenue Growth (yoy) | mayor es mejor | — |
| Crecimiento | Quarterly Earnings Growth (yoy) | mayor es mejor | — |
| Salud Financiera | Total Debt/Equity (mrq) | menor es mejor | solo > 0 |
| Salud Financiera | Current Ratio (mrq) | mayor es mejor | — |
| Dividendo | Forward Annual Dividend Yield (%) | mayor es mejor | solo si 0 < Payout Ratio ≤ 1 |
| Momentum | Retorno 5D, 1M, 3M, 6M, YTD, 1A | mayor es mejor | — |

El filtro de dividendo descarta el yield cuando el payout es > 100 % o no existe
(dividendo no sostenible con utilidades).

### 1.4 Normalización

Para cada métrica, sobre el universo de la corrida:

1. **Winsorización** a los percentiles **p5 / p95** de los valores presentes
   (`WINSOR_PERCENTIL_INFERIOR = 0.05`, `WINSOR_PERCENTIL_SUPERIOR = 0.95`).
2. **z-score** con media y desviación **poblacional** (ddof = 0) de los valores presentes:
   `z = (x − media) / desv`.
3. Si la métrica es "menor es mejor": `z ← −z`.

### 1.5 Score por pilar y score compuesto

- `Score pilar = promedio de los z disponibles` de sus métricas (ignora vacíos).
- `Score Compuesto = Σ (w_p · Score_p) / Σ w_p`, sumando **solo los pilares con dato** en
  esa fila (el peso de un pilar vacío sale del numerador y del denominador).
- `Ranking` = posición por Score Compuesto descendente, empates con método `min`.

Pesos por defecto (`PESOS_PILARES`, inclinados a momentum por decisión del usuario; el
frontend permite cambiarlos con sliders 0–3 y recalcula en el navegador):

| Pilar | Peso | Participación con todos los pilares presentes |
|---|---|---|
| Valor | 0,5 | 8,7 % |
| Calidad | 1,0 | 17,4 % |
| Crecimiento | 1,0 | 17,4 % |
| Salud Financiera | 1,0 | 17,4 % |
| Dividendo | 0,25 | 4,3 % |
| Momentum | 2,0 | 34,8 % |

### 1.6 Reglas de cobertura mínima

| Regla | Efecto |
|---|---|
| Menos de 5 valores válidos en una métrica | No se winsoriza (se usa la serie tal cual) |
| Menos de 2 valores válidos, o desviación = 0 | La métrica completa queda sin z-score (no aporta) |
| Pilar sin ninguna métrica con dato en una fila | Pilar vacío; su peso se redistribuye |
| Fila sin ningún pilar con dato | Score Compuesto vacío y **sin puesto** en el ranking |
| Transparencia | `Métricas Disponibles` / `Métricas Totales` (20) por fila |

### 1.7 Grupos financieros duplicados

Holdings y filiales que consolidan los mismos estados (p. ej. IAM / AGUAS-A) duplicarían la
exposición. `detectar_duplicados_financieros` compara todos los pares por **Revenue (ttm) y
EBITDA**: son "casi iguales" si en ambas columnas `|x − y| / max(|x|, |y|, 1) ≤ 0,001`
(tolerancia relativa de 0,1 %). Los pares se agrupan transitivamente en
`Grupo Financiero Duplicado = 1, 2, …`. **No se eliminan**: se marcan en la tabla y el job
deja una advertencia de riesgo de doble conteo.

---

## 2. Acciones EEUU

Archivos: `internal/modules/us/{universe,scanner,fundamentals,scoring,portfolio}.go`.

### 2.1 Universo y filtro de capitalización

1. **Candidatos** (`universe.go`): unión de los componentes del **S&P 500 y el Nasdaq-100**
   leídos desde Wikipedia (tickers normalizados a formato Yahoo, `BRK.B → BRK-B`). Si ambas
   páginas fallan se usa `FALLBACK_TICKERS`, una lista estática de ~170 large/mega caps.
2. **Market cap** de cada candidato vía `fast_info` (la consulta más liviana), con pausa
   de 0,6 s ± 30 % entre tickers y hasta 2 reintentos.
3. **Filtro**: `market_cap ≥ USD 200.000.000.000` (`DEFAULT_MIN_MARKET_CAP`).

### 2.2 Momentum 12-1 y retornos

Histórico diario de **14 meses** (`yf.download`, `auto_adjust=True`) en una sola descarga.
`P(t)` es el último cierre disponible en o antes de la fecha `t` (`asof`):

- `momentum_12_1 = P(hoy − 30 días) / P(hoy − 365 días) − 1` (excluye el último mes).
- Retornos informativos: 1d, 5d, 3m (91 días), 6m (182 días), 1a (365 días) y YTD
  (desde el 1 de enero), todos `P_hoy / P_t − 1` en días calendario.
- Se exige ≥ 300 días entre el primer y el último dato y precios válidos en −30 y −365 días;
  si no, el ticker queda fuera.

Los candidatos se ordenan por `momentum_12_1` descendente y el **Top N = 15** se toma de ese
orden (el score de salud **no** reordena el top: es información complementaria).

### 2.3 Indicadores técnicos (`fundamentals.go`)

- SMA 50 y SMA 200 (media simple).
- RSI 14 de Wilder: medias exponenciales con `α = 1/14` de ganancias y pérdidas;
  `RSI = 100 − 100 / (1 + RS)`.
- MACD (12, 26, 9): `EMA12 − EMA26`, señal = `EMA9` del MACD, histograma = MACD − señal.
- Volumen del último día y promedio de los últimos 30.

### 2.4 Score de salud (`scoring.go`)

Heurística de reglas 0–100 (no es un modelo estadístico). Cada métrica se lleva a 0–100 con
**interpolación lineal por tramos** (saturada en los extremos); cada categoría es el
promedio de sus métricas disponibles; el total es el promedio ponderado de las categorías
disponibles, renormalizando pesos si falta una categoría entera.

| Categoría | Peso | Métricas (curva, x → puntaje) |
|---|---|---|
| Valoración | 0,25 | P/E trailing (5→100, 15→90, 25→65, 40→35, 60→10; negativo = 10); PEG (0,5→100 … 5→5; ≤ 0 = 30); EV/EBITDA (5→100 … 45→5; negativo = 10); P/B (0,5→90, 1→100, 3→85 … 20→10; negativo = 5) |
| Rentabilidad | 0,25 | ROE (−20 %→0 … 40 %→100); ROA (−10 %→0 … 25 %→100); margen neto (−10 %→0 … 30 %→100) |
| Salud financiera | 0,20 | Deuda/Patrimonio (0→100, 100→70, 400→5; negativo = 5); Current ratio (0,5→15 … 3→100); FCF yield = FCF / market cap (−5 %→5 … 10 %→100) |
| Retorno al accionista | 0,10 | Dividend yield (0→45, 2 %→65, 3,5 %→85, 5 %→100, 9 %→70, 15 %→30: yields muy altos se castigan) |
| Técnico | 0,20 | RSI 14 (máximo 95 en RSI 60; 20→25, 90→15); histograma MACD (−3→10, 0→50, 3→90); tendencia: cuántas de {P > SMA50, P > SMA200, SMA50 > SMA200} se cumplen → 0/1/2/3 = 10/35/70/100 |

Etiquetas: ≥ 75 Fuerte, ≥ 60 Sólido, ≥ 45 Neutral, ≥ 30 Débil, < 30 Muy débil.

### 2.5 Portafolio — modelo único (`portfolio.go`, ver `docs/MODELO_NEW.md`)

Se calcula a pedido (request HTTP) sobre `prices.json` y `candidates.json` de la última
corrida, **sin llamar a Yahoo**. Un solo modelo en tres etapas; el resultado es **long-only**
(pesos ≥ 0 que suman 1). Reemplaza a los tres modelos del original (Markowitz, Black-Litterman
y HRP) y a la búsqueda de la mejor combinación de k tickers.

**Datos comunes**

- Cierres de los tickers elegidos, solo fechas donde **todos** tienen dato; mínimo 2 tickers
  y **30 días comunes** (`MIN_COMMON_DAYS`).
- Retornos diarios simples. Anualización con **252** ruedas:
  `μ = media diaria · 252`, `Σ = cov diaria · 252 + 10⁻⁶ · I` (ridge `COV_RIDGE` para evitar
  matrices casi singulares).
- Tasa libre de riesgo por defecto **4 %** anual (el usuario puede enviar otra entre −5 % y 50 %).

**Etapa 1 — Ranking.** El universo (por defecto, todos los candidatos de la corrida) se ordena
por el score multi-factor de §2.4. Los filtros de liquidez y riesgo fundamental ya los aplica
el scanner del batch.

**Etapa 2 — Diversificación por clusters.** Distancia de correlación
`d_ij = √(0,5 · (1 − ρ_ij))`, clustering jerárquico **average linkage** y corte en `n_assets`
clusters (10 por defecto, 2–20). De cada cluster se elige el ticker de **mayor score** (empate:
orden alfabético). Si el corte da menos clusters que `n_assets`, se completa con los de mayor
score aún no elegidos; si el universo tiene `n_assets` o menos, entran todos.

**Etapa 3 — Black-Litterman sobre los elegidos.**

- Pesos de mercado `w_mkt` proporcionales al market cap (iguales si no hay datos).
- Prior de equilibrio (retorno total): `π = δ · Σ · w_mkt + rf`, con `δ = 2,5`.
- Views automáticas absolutas (P = I), una por ticker, derivadas del score:
  `Q_i = π_i + k · (score_i / 100 − 0,5) · σ_i`, con `k = 0,5` y `σ_i = √Σ_ii`; score
  ausente = 50 (view neutra). El usuario puede sobrescribir cualquier `Q_i`.
- Confianza: `Ω = diag(τ · P Σ Pᵀ) + 10⁻⁸ · I`, con `τ = 0,05`.
- Posterior: `μ_BL = [(τΣ)⁻¹ + PᵀΩ⁻¹P]⁻¹ · [(τΣ)⁻¹π + PᵀΩ⁻¹Q]`.
- Pesos: máximo Sharpe `max_w (wᵀμ_BL − rf) / √(wᵀΣw)` con `Σw = 1` y
  `0 ≤ w ≤ max_weight` (SLSQP desde pesos iguales). `max_weight` es **25 %** por defecto
  (5 %–100 %); si es menor que `1/n_assets` se sube a `1/n_assets`, único valor factible.
- Corrección respecto del original (Fase 7): el original no sumaba `rf` a π y luego la
  restaba en el Sharpe; ahora, con views neutras, se recupera `w_mkt`.

**Métricas del portafolio**: `E[R] = wᵀμ`, `vol = √(wᵀΣw)`, `Sharpe = (E[R] − rf) / vol`
(con `μ` histórico, no `μ_BL`).

---

## 3. ETFs

Archivos: `modules/etf/screener/…` (descarga) y `modules/etf/analysis/…` (análisis).

### 3.1 Screener (`screener/`)

- **Universo**: lista curada `screener/List_All_ETF.csv` (~1.225 tickers con nombre,
  proveedor y categoría Morningstar). La categoría del CSV tiene prioridad sobre la de Yahoo.
  Los tickers que Yahoo ya no reconoce (404 / deslistados, fallidos en corridas sucesivas) se
  sacan de la lista y quedan registrados en `screener/ETF_eliminados.csv` con la fecha de
  eliminación, la última corrida en que validaron y el motivo.
  El proveedor se infiere por palabras clave (iShares, Vanguard, SPDR, Invesco, …; si no,
  "Otro").
- Se procesa en paralelo con el espaciado global de Yahoo (sin pausas fijas).
- **Rendimientos por ventana** (5D, 1M, 6M, YTD, 1Y, 5Y): todos los períodos se derivan de
  **una sola descarga diaria de 5 años** por ETF, y `Performance = (Precio actual / primer cierre del período − 1) · 100`.
- **Riesgo** (sobre el histórico diario de 1 año, `rf = 2 %` constante, 252 ruedas):
  - `Sharpe = (media(r) · 252 − rf) / (desv(r) · √252)`
  - `Beta = cov(r_ETF, r_mercado) / var(r_mercado)` contra **^GSPC**. Los retornos se
    emparejan **por fecha calendario** (join interno). Corrección Fase 7: el original
    alineaba por posición y un día faltante desfasaba toda la serie.
  - `Treynor = (media(r) · 252 − rf) / Beta`
- Salida `etf_perf_data_<timestamp>.csv`. Además, `etf_choice_*` lista hasta 30 ETFs con
  rendimientos **no negativos y no decrecientes** 5D ≤ 1M ≤ 6M ≤ 1Y ≤ 5Y, ordenados por 5Y.

### 3.2 Ranking principal (`ranking.go`)

**Depuración del universo**, en este orden:

1. Solo `TipoInstrumento == "ETF"`.
2. Fuera las categorías `Ultrashort Bond`, `Securitized Bond - Focused`, `Target Maturity`
   (beta ≈ 0 infla Sharpe/Treynor por el denominador).
3. Fuera filas incompletas en: 5D, 1M, 6M, 1Y, Sharpe, Treynor, Beta, 52W High, 52W Low,
   Precio, Volumen, AUM.
4. Liquidez: `Volumen ≥ 10.000` y `AUM ≥ USD 30.000.000`.
   *(Backend Go: `Volumen` es el de la última sesión completa, tomado del gráfico, no el de la sesión en curso; y el AUM puede venir de una
   caché de hasta 28 días. Ver `internal/modules/etf/infocache.go`.)*
5. **Outliers por MAD**: para cada ventana (5D, 1M, 6M, 1Y)
   `z = 0,6745 · (x − mediana) / MAD`, con `MAD = mediana(|x − mediana|)` (si MAD = 0, z = 0).
   Se excluye el ETF si **alguna** ventana tiene `|z| > 6` (`Z_THRESHOLD`). Los excluidos se
   publican con su ventana más atípica y su z, ordenados por z con signo descendente.

**Score compuesto** (0–100). Cada componente es un percentil `rank(pct) · 100` dentro del
universo depurado:

| Componente | Variable | Dirección | Peso |
|---|---|---|---|
| `s_sharpe` | Sharpe Ratio | mayor es mejor | 0,25 |
| `s_treynor` | Treynor Ratio | mayor es mejor | 0,1175 |
| `s_1y` | Performance 1Y | mayor es mejor | 0,14 |
| `s_6m` | Performance 6M | mayor es mejor | 0,075 |
| `s_consistencia` | N° de ventanas (5D, 1M, 6M, 1Y) positivas | mayor es mejor | 0,15 |
| `s_tendencia` | N° de pasos crecientes 5D→1M→6M→1Y (`≥`) | mayor es mejor | 0,10 |
| `s_baja_dispersion` | Desviación estándar entre las 4 ventanas | menor es mejor | 0,05 |
| `s_bajo_beta` | \|Beta\| | menor es mejor | 0,0675 |
| `s_cerca_maximo` | `(Precio − 52W High) / 52W High · 100` | mayor (más cerca del máximo) | 0,05 |

`Score = Σ peso · componente` (los pesos suman 1). El **Top 15** son los 15 de mayor Score.
Cada corrida agrega una fila a `historial_corridas.csv` (universo bruto/depurado, outliers,
Sharpe y Beta promedio, top 1).

### 3.3 Variantes del Top 15

| Variante | Universo | Criterio |
|---|---|---|
| **Top 15 Simple** | Mismo universo depurado | 5 percentiles al 20 % c/u: 5D, 1M, 6M, 1Y (mayor es mejor) y \|Beta\| (menor es mejor) |
| **Top 15 Vanguard** (`service.go`) | Ranking principal filtrado a `Proveedor == "Vanguard"` | Mismo Score, corte a 15 |
| **Top 15 Momentum** (`analyses.go`) | Solo ETF; excluye 22 categorías defensivas (bonos cortos, money market, target maturity, target-date…); exige 4 ventanas + Sharpe/Treynor/Beta. **Sin** filtro de liquidez ni de outliers | `Score_comp` (ver abajo) |
| **Top 15 Cap.** (`analyses.go`) | ETFs en las 9 categorías Large/Mid/Small × Blend/Value/Growth, liquidez igual al ranking principal | 5 mejores por bucket de capitalización (ver abajo) |

**Momentum `Score_comp`.** Con `minmax(x) = 100 · (x − min) / (max − min)` (50 si no hay
rango):

- `M = minmax(0,10 · 5D + 0,20 · 1M + 0,30 · 6M + 0,40 · 1Y)`
- `Q = minmax(0,5 · Sharpe + 0,5 · Treynor_cap)`, con Treynor recortado a sus percentiles
  p1/p99.
- `Consist = minmax(6M_actual − 6M de la corrida archivada más antigua)`; sin histórico, o
  si el ETF no estaba en esa corrida, vale 50.
- `Score_comp = 0,55 · M + 0,25 · Q + 0,20 · Consist`.

**Top 15 Cap.** (inspirado en el Vanguard U.S. Momentum Factor ETF, VFMO, adaptado a ETFs):

- Buckets: Large-cap, Mid-cap y Small-cap según la categoría Morningstar; se excluyen además
  Utilities / Real Estate por defensa, y el propio ETF usado como benchmark.
- `Momentum_Score = (minmax(1Y) + minmax(6M) + minmax(Alpha)) / 3`, con
  `Alpha ≈ 1Y − Beta · 1Y_benchmark` (CAPM de un punto). Benchmark: el primero disponible
  entre **IWV, VTI, SPY**.
- Aproximaciones aceptadas: 1Y y 6M son retornos trailing completos (no excluyen el último
  mes) y el tamaño del fondo usa AUM como proxy de capitalización.
- Selección: **5 por bucket** (15 en total). Peso proporcional a `Momentum_Score`. El tope
  de 1,5 % por ETF es inalcanzable con 15 posiciones (harían falta ≥ 67), así que hoy **no se
  aplica** y se deja un aviso en el log.

### 3.4 Categorías, rotación y archivado

- **Ranking por categoría** (`analyses.go`): sobre el ranking depurado, solo
  categorías con **N ≥ 5** ETFs; ordenadas por **Sharpe promedio** (Treynor por mediana, más
  Beta, 1Y y 6M promedio, Score promedio y % con 1Y positivo).
- **Diversificación**: conteo y % por categoría dentro del Top 15.
- **Rotación** (`rotacion.go`): compara el Top 15 vigente contra la corrida
  archivada más reciente (orden por fecha y secuencia del nombre de archivo, no por fecha de
  modificación):
  - *Mantienen*: puesto actual y previo, `Delta_Rank = previo − actual`, `Delta_Score`, y
    **racha** = número de corridas consecutivas (hacia atrás desde la vigente) en que el ETF
    estuvo en el Top 15.
  - *Nuevos*: con su procedencia (estaba excluido por outlier, estaba en el ranking en el
    puesto #n, o no estaba en el universo).
  - *Salidas*: con su destino (pasó a outliers, su categoría se excluyó, bajó al puesto #n, o
    dejó el universo).
- **Archivado** (`storage.go`, `pipeline.go`): el análisis corre primero en una
  carpeta de *staging*; solo si termina sin errores se archiva la corrida vigente en
  `data/etf/hist` (sufijo `_AAAAMMDD` y contador `_2`, `_3`… si ya hay una corrida archivada
  ese día, para no perderla) y se instala la nueva. Las salidas crudas del screener de más de
  30 días se mueven (no se borran) a `screener/data_old/`.

---

## 4. Tendencia (Asia + EEUU)

Archivos: `internal/modules/tend/{params,discovery,universe,dataset,markowitz,forecast,momentum}.go`.

### 4.1 Universo dinámico (`discovery.go`)

El universo se descubre en cada corrida con el screener de Yahoo (`yf.screen`, orden por
`intradaymarketcap`) y se guarda en `data/tendencia/universe/universe_<AAAAMMDD>.json`; si
ya hay un snapshot del día, se reutiliza. **Backend Go:** reutiliza el snapshot si tiene hasta
3 días (`UniverseMaxAgeDays`); si es más viejo, no existe, o se pasa `--force-discovery`, lo
redescubre. Con `--offline`/`--skip-discovery` nunca consulta Yahoo. Si el descubrimiento falla
conserva el snapshot viejo (con aviso en el log) en vez de usar la lista de respaldo.

- **Asia**: regiones `cn` (China), `hk` (Hong Kong), `jp` (Japón), `kr` (Corea del Sur),
  `tw` (Taiwán), `in` (India). Se piden **20 por región**, se deduplica por empresa (nombre
  normalizado sin sufijos societarios, primeros 20 caracteres; gana el listado de mayor
  capitalización, p. ej. A-shares vs H-shares) y se toman las **20 más grandes en conjunto**
  (sin cuota por país).
- **EEUU**: se piden 150 candidatas de la región `us` y quedan **todas las mega-caps con
  capitalización ≥ USD 200 mil millones** (tamaño variable, no un top-N fijo).
  **Filtro de sede**: se descartan las que tienen sede (`assetProfile.country`) distinta de "United States"
  (ADRs de TSMC, Samsung, Toyota…); las de sede desconocida se conservan.
- Sector desde `quoteSummary/assetProfile` (el screener no lo trae).
- Se excluyen tickers cuya moneda no tenga par FX (ver 4.2).
- **Fallback**: si el screener falla o devuelve menos de 10 empresas en Asia o en EEUU, se
  usa la lista estática de respaldo (`fallback_gen.go`) y el snapshot queda marcado
  `fallback_static`.

### 4.2 Precios y conversión a USD

- Precios **semanales de 5 años** (`period="5y"`, `interval="1wk"`) del universo más los
  tickers de referencia `^VIX` y `^IRX`, con caché en disco y reintentos con backoff.
- FX: pares `USDXXX=X` (unidades locales por 1 USD). Se acepta **cualquier moneda ISO 4217**
  (`internal/fx`). Las subunidades de
  Yahoo se dividen por 100: `GBp`/`GBX` → GBP, `ZAc`/`ZAC` → ZAR, `ILA` → ILS.
- `precio_USD = precio_local / FX / divisor`. El FX se alinea al calendario del precio con
  **forward-fill** (último tipo de cambio conocido, sin mirar al futuro).

### 4.3 Armado del dataset (`dataset.go`)

1. Cada serie se lleva a una fecha por semana (sin duplicados).
2. Ticker excluido si: no hay datos, falta su FX, o tiene menos de
   **200 semanas** (`MIN_WEEKS_REQUIRED`, ~4 años) de precios en USD.
3. Matriz con la unión de fechas y `ffill(limit=2)` (máx. 2 semanas) para feriados y
   desalineación entre mercados.
4. Retornos semanales simples. Los excluidos y el motivo quedan en `excluded_tickers.txt`.
5. **Tasa libre de riesgo** (anual, decimal): Treasury 3M (`DGS3MO`) del último
   `macro_data_*.csv` de FRED → si no, último valor de `^IRX` / 100 → si no, **4 %**.

### 4.4 Selección Markowitz (`markowitz.go`)

- `μ` = `retorno histórico medio anualizado (52 semanas)`: **CAGR** geométrico,
  `(P_final / P_inicial)^(52 / n_semanas) − 1`.
- `Σ` = `sample_cov(frequency=52)`: covarianza muestral de retornos semanales × 52.
- **Paso 1 — universo completo**: máximo Sharpe con `rf`, long-only, tope
  **`MAX_WEIGHT_PER_ASSET = 15 %`** por activo y regularización **L2 con γ = 0,15** (evita que
  la solución colapse en 2–3 activos). Si `max_sharpe` falla, se usa `min_volatility`.
- **Paso 2 — Top N** (`TOP_N_SELECTION = 15`): los N de mayor peso (> 10⁻⁶); si el
  optimizador dejó menos de N con peso, se completa con los de mayor Sharpe individual
  `(μ_i − rf) / σ_i`.
- **Paso 3 — portafolio final**: se re-optimiza máximo Sharpe solo sobre esos N, con tope
  `max(30 %, 1/N)` y L2 γ = 0,05.
- **Portafolios reducidos 5 y 10** (`PORTFOLIO_SIZE_OPTIONS`): los 5 / 10 primeros del Top N
  (por peso del paso 1), re-optimizados igual que el paso 3 para que repartan el 100 % entre
  ellos.
- **Análisis "solo EEUU"**: el mismo modelo corrido **desde cero** sobre el universo de EEUU
  (no es un filtro del combinado), con `TOP_N_SELECTION_US = 15`; se omite si hay menos de
  **5** tickers de EEUU con datos.
- **Validación previa** (`validar_configuracion`): la corrida se rechaza si
  `top_n · MAX_WEIGHT_PER_ASSET < 1` (los pesos no podrían sumar 100 %), si `top_n` supera el
  universo o si un portafolio reducido no es menor que `top_n`.

### 4.5 Señales de tendencia (`forecast.go`)

Sobre las acciones seleccionadas, con medias móviles de precios semanales en USD:

| Señal | Medias | Regla |
|---|---|---|
| Señal 6m | 10 vs 40 semanas | **Alcista** si MA_corta > MA_larga y P ≥ MA_corta; **Bajista** si MA_corta < MA_larga y P ≤ MA_corta; si no, **Lateral / mixta** |
| Señal 3m | 4 vs 13 semanas | Misma regla |

Además, `momentum 26s = P_t / P_{t−26} − 1`. Las señales son **solo informativas**: no
entran en Markowitz ni en la proyección.

### 4.6 Proyección a 6 meses (GBM)

Horizonte `T = 26 / 52 = 0,5` años; `μ_i` = CAGR y `σ_i = √Σ_ii` (anual):

- Mediana: `P_6m = P · (1 + μ_i)^T = P · exp(ln(1 + μ_i) · T)`
- Banda 90 %: `P · exp(ln(1 + μ_i) · T ± 1,645 · σ_i · √T)`

Corrección Fase 7: el original usaba `exp((μ − σ²/2) · T)`, tratando el CAGR como tasa
continua y descontando dos veces el *volatility drag*. La proyección es un **rango de
escenarios** basado en 5 años de historia, no un pronóstico.

**Notas de régimen** (`market_regime_notes.txt`): VIX ≥ 25 volatilidad elevada, ≤ 15 baja,
entre medio normal; spread 10Y-2Y < 0 curva invertida; spread High Yield como referencia de
apetito por riesgo (estos dos últimos solo si hay datos FRED).

### 4.7 Momentum mensual y semanal (`momentum.go`)

Sobre **todo** el universo de cada análisis (no solo la selección Markowitz):

| | Mensual | Semanal |
|---|---|---|
| Vela | último precio del mes calendario | último precio de la semana |
| Ventanas de retorno | 1, 3, 6, 12 meses | 1, 2, 3, 4 semanas |
| Consistencia | % de meses positivos en los últimos 12 | % de semanas positivas en las últimas 4 |
| Historia mínima | 13 meses | 5 semanas |

`momentum_score = promedio(4 retornos) · (0,5 + 0,5 · consistencia)` (consistencia 0,5 si
falta). Si falta alguna ventana, el ticker no se rankea. Se publican los **10** mejores
(`TOP_N_MOMENTUM`). No es una medida ajustada por riesgo.

### 4.8 Parámetros por corrida

`fogradar job tendencia` acepta `--top-n`, `--top-n-us` y `--max-weight`, que
sobrescriben `TOP_N_SELECTION`, `TOP_N_SELECTION_US` y `MAX_WEIGHT_PER_ASSET` **solo para esa
corrida** (se validan con la regla de 4.4). Cada corrida escribe en
`output/<AAAAMMDD>_<NN>/run_manifest.json` los parámetros efectivos (`top_n`, `top_n_us`,
`max_weight_per_asset`, `l2_gamma`, `min_weeks_required`, `universe_source`) y, si hubo,
`parametros_sobrescritos`.

---

## 5. Funciones transversales

Archivo: `internal/transversal/transversal.go`.

**Frescura del hub.** Para cada módulo con programa asignado:

```
esperada = última ejecución programada en o antes de (ahora − 3 h)
frescura = "al_dia"   si datos_al ≥ esperada
           "atrasado" en otro caso
```

El margen de 3 h (`MARGEN_CORRIDA`) da tiempo a que la corrida termine (ETF ~30 min, la
secuencia diaria ~25 min, más reintentos). Ejemplo: un martes a las 19:00 la referencia es
`martes 16:00 → lunes 18:00`; recién desde las 21:00 del martes se exigen datos del martes.
Sin datos: `sin_datos`; datos ilegibles: `error` (el hub nunca se cae por un módulo).

**Buscador global.** La consulta se normaliza (sin tildes, minúsculas, espacios simples) y
se compara con el índice de cada módulo (caché de 60 s). Orden de resultados:

1. Coincidencia exacta de ticker, o del ticker sin sufijo de bolsa (`VAPORES.SN` → `vapores`,
   `^VIX` → `vix`).
2. Ticker que empieza con la consulta.
3. Nombre que contiene la consulta (solo si tiene ≥ 3 caracteres).

Dentro de cada grupo: por módulo (Chile, EEUU, ETF, Tendencia) y luego por posición en el
ranking de ese módulo. Máximo 50 resultados.

---

## 6. Limitaciones y supuestos comunes

- **Sesgo de supervivencia.** Los universos se arman con lo que existe *hoy* (IPSA actual,
  S&P 500/Nasdaq-100 vigentes, lista curada de ETFs, mega-caps del día). Las empresas que
  salieron de los índices o dejaron de cotizar no aparecen, lo que infla los retornos
  históricos.
- **Estimación dentro de la muestra.** Los retornos esperados y las covarianzas (EEUU,
  Tendencia) se estiman con el mismo historial que se usa para elegir; los portafolios de
  máximo Sharpe son muy sensibles a μ y suelen decepcionar fuera de la muestra. La
  regularización L2 y los topes por activo mitigan, pero no eliminan, este problema.
  No hay backtest fuera de muestra integrado.
- **Scores relativos.** Los z-scores (Chile) y percentiles o min-max (ETF) comparan solo
  contra el universo de la corrida: un "buen" puntaje no significa un buen valor absoluto.
  Los pesos de pilares y componentes son decisiones de estilo, no parámetros estimados.
- **Calidad de datos de Yahoo Finance.** Los fundamentales pueden venir desactualizados,
  incompletos o en otra moneda; algunos endpoints (screener) no están documentados y pueden
  cambiar. Cada módulo tiene respaldos (listas estáticas, reglas de cobertura), pero un dato
  erróneo de Yahoo se propaga al ranking.
- **Supuestos simplificadores.** Tasas libres de riesgo constantes (4 % EEUU, 2 % ETF);
  aproximaciones de momentum y alpha en Top 15 Cap.; proyección GBM con parámetros
  históricos constantes; sin costos de transacción, impuestos ni restricciones de liquidez en
  los portafolios.
- **No es asesoría de inversión.** Fog Radar Quant es una herramienta de análisis personal y
  educativa. Ningún ranking, portafolio ni proyección constituye una recomendación de compra
  o venta.
