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
export type Loaded<T> =
	{ ok: true; data: T } | { ok: false; message: string; code: string; status?: number };

// --- Módulo Chile (IPSA) ---
export type ClRankingResponse = Schemas['RankingResponse'];
export type ClRankingRow = Schemas['RankingRow'];
export type ClPilarInfo = Schemas['PilarInfo'];
export type ClPilarKey = ClPilarInfo['key'];
export type ClIndicadoresResponse = Schemas['IndicadoresResponse'];
export type ClIndicadorRow = Schemas['IndicadorRow'];
export type ClColumnaInfo = Schemas['ColumnaInfo'];
export type ClUnidad = ClColumnaInfo['unidad'];

// --- Módulo EEUU ---
export type UsRunResponse = Schemas['RunResponse'];
export type UsCandidateRow = Schemas['CandidateRow'];
export type UsScore = Schemas['Score'];
export type UsScoreCategories = Schemas['ScoreCategories'];
export type UsPriceMatrix = Schemas['PriceMatrix'];
export type UsTickerEvaluation = Schemas['TickerEvaluation'];
export type UsPortfolioMethod = Schemas['PortfolioRequest']['method'];
export type UsPortfolioRequest = Schemas['PortfolioRequest'];
export type UsPortfolioResult = Schemas['PortfolioResult'];
export type UsBestCombinationRequest = Schemas['BestCombinationRequest'];
export type UsBestCombinationResult = Schemas['BestCombinationResult'];
export type UsRankBy = Schemas['BestCombinationRequest']['rank_by'];
export type UsPortfolioLimits = Schemas['PortfolioLimits'];

// --- Módulo ETF ---
export type EtfFila = Schemas['EtfFila'];
export type EtfTabla = Schemas['EtfTabla'];
export type EtfResumen = Schemas['EtfResumen'];
export type EtfModelo = Schemas['EtfModelo'];
export type EtfPeso = Schemas['EtfPeso'];
export type EtfHistorialFila = Schemas['EtfHistorialFila'];
export type EtfRotacion = Schemas['EtfRotacion'];
export type EtfRotacionFila = Schemas['EtfRotacionFila'];
export type EtfDiversificacion = Schemas['EtfDiversificacion'];
export type EtfCategorias = Schemas['EtfCategorias'];
export type EtfCategoriaFila = Schemas['EtfCategoriaFila'];
export type EtfCap = Schemas['EtfCap'];
export type EtfCapFila = Schemas['EtfCapFila'];
export type EtfCapBucket = Schemas['EtfCapBucket'];

// --- Módulo Tendencia ---
export type TdDashboard = Schemas['TdDashboard'];
export type TdAnalisis = Schemas['TdAnalisis'];
export type TdHolding = Schemas['TdHolding'];
export type TdSector = Schemas['TdSector'];
export type TdMomentumMensual = Schemas['TdMomentumMensual'];
export type TdMomentumSemanal = Schemas['TdMomentumSemanal'];
export type TdPortafolioReducido = Schemas['TdPortafolioReducido'];
export type TdParametros = Schemas['TdParametros'];
export type TdArchivo = Schemas['TdArchivo'];
export type TdRunPerformance = Schemas['TdRunPerformance'];
export type TdSectorHistoria = Schemas['TdSectorHistoria'];
export type TdTickerHistoria = Schemas['TdTickerHistoria'];
export type TdTickerNombre = Schemas['TdTickerNombre'];
export type TdMomentumLider = Schemas['TdMomentumLider'];
export type TdAnalisisDisponible = Schemas['TdAnalisisDisponible'];
export type TdExcluido = Schemas['TdExcluido'];
