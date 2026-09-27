# Checklist de paridad por módulo

Un módulo se da por migrado cuando, con los mismos datos, el sitio nuevo muestra lo mismo que
el proyecto original. Las diferencias intencionales quedan anotadas.

## Chile / IPSA (`Stock_Choice_Nacional`) — ✅ 2026-09-27

| Ítem | Cómo se verificó | Estado |
|---|---|---|
| Ranking (scoring) | `tests/cl_stocks/test_parity.py`: indicadores del 24-09 y del 25-09 → `ranking_acciones_*.csv` **byte a byte idéntico** al generado por `scoring_acciones.py` | ✅ |
| Orden y puesto (empates `min`) | Mismo test, fila a fila (30 acciones) | ✅ |
| Recálculo con pesos en el navegador | `src/lib/modules/cl/scoring.spec.ts`: con los pesos por defecto reproduce orden, puesto y score del backend (±0,001 por el redondeo a 4 decimales del CSV) | ✅ |
| Duplicados holding/filial | Test unitario + badge "grupo N" en la tabla y aviso en el resumen | ✅ |
| Dividendo sostenible (payout ≤ 100 %) | Test unitario | ✅ |
| Conversión de moneda del extractor | `tests/cl_stocks/test_extractor.py` con dobles de `yf.Ticker` (sin red) | ✅ |
| Endpoint ranking | `/api/ranking` → `/api/cl/ranking` (claves snake_case, incluye `modelo`) | ✅ |
| Endpoint indicadores | `/api/indicadores` → `/api/cl/indicadores` (catálogo de 61 columnas con unidad) | ✅ |
| KPIs (acciones, mejor score, cobertura, duplicados) | Visual, contra el original | ✅ |
| Panel de pesos (0–3, paso 0,25, % del score, Restablecer) | E2E `chile.e2e.ts` | ✅ |
| Búsqueda por símbolo/nombre y filtro por sector | E2E | ✅ |
| Detalle por acción (4 grupos de métricas) | E2E | ✅ |
| Pipeline completo contra Yahoo | Pendiente: correr en Windows (`uv run python -m fog_radar.jobs cl_stocks`) | ⏳ |

**Diferencias intencionales**

- El filtro de sector ya **no re-numera** el ranking: el puesto se calcula sobre todo el
  universo y el filtro solo oculta filas (en el original, filtrar recalculaba los puestos
  dentro del subconjunto).
- Se agregó: vista `/chile/indicadores` (el original tenía el endpoint pero no la vista),
  top 10 en gráfico, exportación CSV, fecha de los datos, modo oscuro.
- Las acciones sin ningún pilar con dato quedan con puesto vacío (en el original tomaban el
  número de fila).
