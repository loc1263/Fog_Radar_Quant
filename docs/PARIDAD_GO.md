# Paridad del backend Go con Python

Python es el oráculo. Este documento lista **cómo se verifica** la paridad y las **diferencias intencionales** del backend Go
(`backend-go/`). Cuando el corte se haga, estas diferencias deben pasar a `docs/PARIDAD.md` («Correcciones cuantitativas»).

## Cómo se verifica
| Qué | Herramienta |
|---|---|
| Respuestas JSON (110 caminos, incluye 404/422, `POST optimize`, hub y buscador) | `backend-go/golden/compare.py` |
| Forma del OpenAPI (rutas, parámetros, códigos, cuerpos) | `backend-go/golden/openapi_diff.py` |
| Tipos TS generados (`schema.d.ts`) | `npx openapi-typescript` sobre el OpenAPI de Go vs `frontend/src/lib/api/schema.d.ts` |
| Chile, ETF, EEUU, Tendencia (archivos) | tests Go contra los fixtures de `backend/tests/<modulo>/fixtures` |
| Tendencia de punta a punta | misma entrada offline en Go y Python; comparar `processed/` y `output/` y `history.db` |

## Diferencias intencionales
1. **Empates en ordenamientos.** pandas/numpy usan un sort inestable con SIMD (AVX512) cuyo orden entre empates no es
   reproducible; Go usa orden estable. Afecta a filas con el mismo valor (p. ej. pesos 0 en `markowitz_universe_weights`,
   `universe_weights.universe_rank`, empates en rankings ETF). Los valores no cambian; los tests comparan por clave.
2. **Formato de floats en JSON.** Python imprime `5.0`, Go `5`. Mismo valor.
3. **Textos de 422.** Los mensajes de validación de parámetros vienen de huma, no de FastAPI (mismo código y estructura `{detail, code}`).
4. **ETF — rendimientos por período.** El screener los deriva de **un** gráfico diario de 5 años (en vez de una llamada por
   período): menos requests; las columnas coinciden con las de Python en los fixtures.
5. **Chile — pausas.** No hay pausa fija de 30 s cada 15 tickers: el espaciador global de Yahoo ya limita el ritmo.
   Pipeline completo 43 s vs 115 s.
6. **EEUU — lista de respaldo.** Se limpia (`BRK.B` → `BRK-B`).
7. **EEUU — max-Sharpe sin exceso positivo sobre la tasa libre.** Go devuelve un error explícito; el SLSQP de Python devolvía un
   portafolio sin sentido.
8. **Optimizador.** QP propio (ADMM, transformación de Cornuéjols–Tütüncü) en vez de cvxpy; reproduce PyPortfolioOpt con L2 y topes
   a ~1e-10 en los fixtures. Si max-Sharpe falla, cae a mínima volatilidad **sin** L2 (igual que Python).
9. **Tendencia — screener.** `gohoo` no expone el screener de Yahoo: Go reutiliza el snapshot de hoy o el último (como
   `--skip-discovery`) y, sin ninguno, la lista fija de respaldo. El descubrimiento sigue en Python.
10. **PDF.** El de ETF se genera con `go-pdf/fpdf` (mismas secciones, columnas, paleta y fuente Carlito; el maquetado no es idéntico
    al de ReportLab: mismos anchos de columna y tamaños, el salto de página puede variar). El de Tendencia dibuja los gráficos de barras directamente (sin matplotlib).
11. **Mapeo de campos de Yahoo.** `dividendYield` ×100; `forwardPE` desde `summaryDetail` (el de `defaultKeyStatistics` viene
    corrupto en algunas acciones chilenas); `trailingPegRatio` desde fundamentals-timeseries; `netExpenseRatio` =
    `fundProfile.feesExpensesInvestment.annualReportExpenseRatio × 100`.
12. **Redondeo.** `numpy.round` y `round()` de Python difieren; Go tiene los dos (`quant.Round` y el redondeo decimal correcto).

## Tolerancias
Pesos y métricas de Tendencia: `rtol 1e-7`. Los pesos (5 decimales), textos y CSV de Chile/ETF son idénticos byte a byte.
