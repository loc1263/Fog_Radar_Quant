/**
 * Tipos de la API del backend, generados desde su OpenAPI.
 * Regenerar con `npm run gen:api` (backend corriendo en :8000).
 */
import type { components, paths } from './schema';

export type { paths };
export type Schemas = components['schemas'];

export type HealthResponse = Schemas['HealthResponse'];
export type SystemStatus = Schemas['SystemStatus'];
export type ModuleStatus = Schemas['ModuleStatus'];
export type JobRecord = Schemas['JobRecord'];
export type ProgramaInfo = Schemas['ProgramaInfo'];
export type ErrorResponse = Schemas['ErrorResponse'];

// --- Transversal: hub y buscador global ---
export type HubResponse = Schemas['HubResponse'];
export type HubModulo = Schemas['HubModulo'];
export type HubItem = Schemas['HubItem'];
export type Frescura = HubModulo['frescura'];
export type Unidad = HubModulo['unidad'];

/** Resultado de `datos` (lib/datos.ts): datos o error mostrable. */
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
export type UsPortfolioResult = Schemas['PortfolioResult'];
export type UsPortfolioPick = Schemas['PortfolioPick'];
export type UsPortfolioPrecalc = Schemas['PortfolioPrecalcResponse'];

// --- Módulo ETF ---
export type EtfFila = Schemas['EtfFila'];
export type EtfTabla = Schemas['EtfTabla'];
export type EtfResumen = Schemas['EtfResumen'];
export type EtfModelo = Schemas['EtfModelo'];
export type EtfPeso = Schemas['EtfPeso'];
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
export type TdRetornoReciente = Schemas['TdRetornoReciente'];
export type TdPortafolioReducido = Schemas['TdPortafolioReducido'];
export type TdParametros = Schemas['TdParametros'];
export type TdExcluido = Schemas['TdExcluido'];
