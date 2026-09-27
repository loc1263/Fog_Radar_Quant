/**
 * Tipos de la API de FastAPI, generados desde su OpenAPI.
 * Regenerar con `npm run gen:api` (backend corriendo en :8000).
 */
import type { components, paths } from './schema';

export type { paths };
export type Schemas = components['schemas'];

export type HealthResponse = Schemas['HealthResponse'];
export type SystemStatus = Schemas['SystemStatus'];
export type ModuleStatus = Schemas['ModuleStatus'];
export type JobRecord = Schemas['JobRecord'];
export type ErrorResponse = Schemas['ErrorResponse'];

/** Resultado de `tryCall` (lib/server/safe.ts): datos o error mostrable. */
export type Loaded<T> = { ok: true; data: T } | { ok: false; message: string; code: string };

// --- Módulo Chile (IPSA) ---
export type ClRankingResponse = Schemas['RankingResponse'];
export type ClRankingRow = Schemas['RankingRow'];
export type ClPilarInfo = Schemas['PilarInfo'];
export type ClPilarKey = ClPilarInfo['key'];
export type ClIndicadoresResponse = Schemas['IndicadoresResponse'];
export type ClIndicadorRow = Schemas['IndicadorRow'];
export type ClColumnaInfo = Schemas['ColumnaInfo'];
export type ClUnidad = ClColumnaInfo['unidad'];
