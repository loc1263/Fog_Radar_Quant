# Fase 0 — Línea base y spikes de la migración a Go

*Fecha: 2026-10-08 · Código en `backend-go/spikes/` y `backend-go/golden/`. Ver `roadmap_go.md`.*

## Veredicto

| Spike | Resultado | Decisión |
|---|---|---|
| **A — TLS / bloqueo de Yahoo con `net/http`** | ✅ 84 requests (42 tickers × chart + quoteSummary) con el espaciado actual: **0 respuestas 429/401/403**. Más 250 requests de estrés: 0 errores. | No hace falta `utls` por ahora. Repetir el test en la Fase 2 con volumen de pipeline completo (~1000 requests) y desde el servidor de producción (otra IP). |
| **B — Equivalencia de datos gohoo vs. yfinance** | ✅ Cierres y nº de barras idénticos (42 tickers: EEUU, Chile `.SN`, ETF, Japón, HK, UK, AU, Suiza, FX, índices, BTC); diferencias de ~1e-4 solo en la última barra (sesión en curso). ⚠️ `info`: ver abajo. | Capa de mapeo `info` yfinance → quoteSummary en la Fase 2 (con tests por campo). |
| **C — Optimizador max-Sharpe (PyPortfolioOpt)** | ✅ QP propio (ADMM tipo OSQP sobre gonum): universo de 75 activos y portafolio final de 15 → **|Δw| = 1.6e-10 (crudo) y 0 tras `clean_weights`**, en **25 ms** y 0.4 ms. | Riesgo del optimizador retirado. Falta HRP (`linkage`) y Black-Litterman en la Fase 3. |

### Spike B — detalle de `info`
- `dividendYield`: yfinance lo entrega en **porcentaje** (0.32) y quoteSummary en **fracción** (0.0032). Regla de mapeo: ×100.
- `forwardPE`: difiere en 5 tickers chilenos (p. ej. COPEC.SN 10.8 vs 11454; ENELAM.SN 8.7 vs 8991): mi aplanado
  tomó la primera ocurrencia entre módulos. yfinance usa `defaultKeyStatistics.forwardPE`; hay que fijar el módulo
  de cada campo. Investigar en la Fase 4 (Chile usa estos campos).
- Campos ausentes en gohoo y presentes en yfinance: `dividendYield` (10 ETFs/acciones sin el campo en
  `summaryDetail`), `priceToBook` (6), `forwardPE` (3). yfinance rellena con claves alternativas
  (`trailingAnnualDividendYield`, etc.); replicar esa cascada.
- Pendiente de contrastar: fundamentales trimestrales (`FundamentalsTimeseries` vs `quarterly_income_stmt`/`quarterly_cashflow`),
  que usa el extractor de Chile.

## Línea base de los pipelines (corridas reales en `data/_jobs`)

| Pipeline | Duración típica | Requests a Yahoo | Espera por espaciado |
|---|---|---|---|
| cl_stocks | 87–117 s | 182 | 31 s |
| us_stocks | 761 s (máx. 992) | 1 008 | 117 s |
| tendencia | 618 s | — | — |
| etf | 1 440 s (máx. 1 494) | — | — |

**Hallazgo (corrige `ANALISIS_MIGRACION_GO.md`):** el espaciado explícito explica solo ~15 % del tiempo de EEUU
(117 s de 761 s). El resto es **latencia de red en serie**: cada request espera su respuesta (~0.25 s) antes de que
empiece el siguiente. Throughput actual de EEUU: 1 008 / 761 ≈ **1.3 req/s**.

Medido con gohoo (`spikes/stress`, 5 años semanal, intervalo mínimo entre *inicios* de request):

| Configuración | Throughput | 429 |
|---|---|---|
| 1 worker, 475 ms (espaciado actual medio) | 2.11 req/s | 0 / 100 |
| 4 workers, 350 ms (cota inferior actual) | 2.86 req/s | 0 / 150 |

Espaciar **inicios** en vez de esperar al término ya da ~1.6×; con 4 workers ~2.2×, sin tocar los límites de
Yahoo. Proyección para EEUU (1 008 requests): ~760 s → **~350 s**; ETF (1 440 s) en proporción, según su nº de requests.
Bajar el intervalo de 350 ms es posible solo con más pruebas (riesgo de 429); no se asume.

## Línea base de la API (47 GET sin parámetros, `golden/capture.py`)

Mediana **13 ms**, p95 **28 ms**, máx. 487 ms (datos reales, TestClient en proceso). La API ya es rápida: la
"lentitud" percibida viene de los pipelines, no de los endpoints. Los 24 endpoints de ETF respondieron 503
(`PermissionError` al leer `etf_top15_recomendados.csv`, probablemente un archivo abierto/bloqueado en Windows); hay
que **volver a capturar ETF** antes de dar la Fase 6 por comparable.

Respuestas congeladas en `backend-go/golden/python/*.json` (oráculo de paridad; 47 archivos).
Los pipelines se comparan con los fixtures ya existentes en `backend/tests/<modulo>/fixtures` (Tendencia: corrida
`20260926_01` completa).

## Cierre de los pendientes de la Fase 0

### 1. ETF: `PermissionError` — causa raíz encontrada (bug de producción, corregido)
- **Síntoma:** los 24 endpoints de ETF daban 503 y la corrida del 7-oct falló al final (24 min perdidos) con
  `PermissionError` al leer `data/etf/current/historial_corridas.csv`.
- **Causa:** `storage.preparar_staging` usaba `tempfile.mkdtemp`. En Windows con Python ≥ 3.12 esa carpeta se crea
  con una ACL **sin herencia** (solo SYSTEM, Administradores y OWNER RIGHTS); los archivos copiados ahí y movidos a
  `current/` conservan esa ACL y quedan ilegibles para el usuario normal (si el proceso corrió elevado). Verificado
  con `icacls`: un archivo movido desde `mkdtemp` no tiene `loc12`; uno desde `mkdir` normal sí.
- **Corrección (`modules/etf/storage.py`):** carpeta de staging con `mkdir` + `uuid` (hereda la ACL de `data/etf`).
  Los 13 tests de ETF que fallan en Windows (comparación byte a byte con CRLF) fallan igual sin el cambio: no es regresión.
- **Pendiente del usuario (requiere administrador):** los archivos ya afectados (`data/etf/current/*.csv` del 5-oct)
  siguen bloqueados. Desde un PowerShell **como administrador**:
  `takeown /f D:\Claude\Fog_Radar_Quant\data\etf /r /d s` y luego `icacls D:\Claude\Fog_Radar_Quant\data\etf /reset /t /c`.
  Después se recapturan los goldens de ETF con `golden/capture.py`.
- **Lección para Go:** crear carpetas temporales con `os.Mkdir` bajo el directorio de datos (no `os.MkdirTemp` en `%TEMP%`
  seguido de mover), y probar la lectura posterior como parte del test de pipeline en Windows.

### 2. Fundamentales de Chile — ✅ equivalentes
`FundamentalsTimeseries` (`quarterlyTotalRevenue`, `quarterlyCapitalExpenditure`) frente a `quarterly_income_stmt` /
`quarterly_cashflow` de yfinance, para los **30 tickers del IPSA**: el crecimiento de ingresos YoY y el CapEx de los
4 últimos trimestres (los dos cálculos del extractor) son **idénticos en 30/30** (tolerancia 1e-9). Yahoo entrega 5
trimestres, igual que yfinance; tickers sin CapEx (ENELCHILE, VAPORES con 1) dan `None` en ambos.

### 3. Perfil de Tendencia y ETF (corrida del 7-oct, `data/_jobs/logs`)

| Pipeline | Etapa | Duración | Naturaleza |
|---|---|---|---|
| EEUU (762 s) | Paso 2: market cap de 503 tickers (`Ticker.info`) | **679 s (89 %)** | red en serie, 1.35 s/ticker |
| EEUU | Pasos 4–7 (históricos, indicadores, fundamentales, score) | ~80 s | red + cómputo |
| Tendencia (618 s) | Paso 0: descubrimiento del universo (screener + `info` de ~110 tickers) | **370 s (60 %)** | red en serie |
| Tendencia | Paso 1: precios de 79 tickers | ~230 s (37 %) | red en serie (~3 s/ticker) |
| Tendencia | Pasos 2–8: FX, dataset, Markowitz, forecast, momentum, PDF | ~20 s (3 %) | **cómputo** |
| ETF (1 440 s) | Screener de 1 226 tickers | ~1 440 s (>99 %) | red (con hilos y pausas anti-429) |
| ETF | Análisis, archivado, informe | < 5 s | cómputo |

**Conclusión:** el cómputo (pandas/scipy/Markowitz) es **≈ 3 %** del tiempo total de los pipelines; el 95–99 % es
red. Reescribir el cálculo en Go ahorra segundos; lo que reduce minutos es el cliente Yahoo (concurrencia acotada,
menos requests por ticker, `quoteSummary` con solo los módulos necesarios). `gohoo` mide ~0.25 s por `quoteSummary`
frente a 1.35 s por `Ticker.info` en yfinance (que dispara varios requests por llamada).

### 4. Spike A a escala — ✅
500 tickers del universo ETF real (`List_All_ETF.csv`), cada uno con `quoteSummary` (price, summaryDetail,
defaultKeyStatistics) + `chart` 1 año = **1 000 requests en 350 s (2.86 req/s), 4 workers, inicios espaciados 350 ms:
0 respuestas 429, 0 errores**, sin reintentos (política `MaxAttempts=1` para detectar cualquier 429). Equivale a una
corrida completa de EEUU en ~6 min frente a 12.7 min hoy. *(Una primera pasada con una lista mal parseada produjo
404 espurios; se descartó y repitió.)* **Sigue abierto:** repetir desde la IP del servidor de producción, que no es
accesible desde este equipo.

## Pendientes que quedan
- [ ] **Usuario (administrador):** desbloquear `data/etf` con `takeown`/`icacls` (ver punto 1) y recapturar los
  goldens de ETF con `backend-go/golden/capture.py`.
- [ ] Repetir el Spike A desde el servidor de producción.
