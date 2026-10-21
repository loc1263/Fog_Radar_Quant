/** Formateo y reglas de presentación del módulo EEUU (port de `lib/format.ts` del original). */
import type { UsCandidateRow, UsScoreCategories } from '$lib/api/types';
import { fmtNumber, fmtPct } from '$lib/format';

const EMPTY = '—';

function isNum(v: unknown): v is number {
	return typeof v === 'number' && Number.isFinite(v);
}

/** Montos en USD compactos, como el original: $1,15T · $262,4B · $15,2M. */
export function fmtUsd(v: unknown): string {
	if (!isNum(v)) return EMPTY;
	const abs = Math.abs(v);
	if (abs >= 1e12) return `$${fmtNumber(v / 1e12, 2)}T`;
	if (abs >= 1e9) return `$${fmtNumber(v / 1e9, 1)}B`;
	if (abs >= 1e6) return `$${fmtNumber(v / 1e6, 1)}M`;
	return `$${fmtNumber(v, 0)}`;
}

export function fmtPrice(v: unknown): string {
	return isNum(v) ? `$${fmtNumber(v, 2)}` : EMPTY;
}

/** Retornos con signo y 2 decimales (+12,34 %). */
export function fmtRet(v: unknown): string {
	return fmtPct(v, { digits: 2, signed: true });
}

/** Proporciones sin signo (pesos, volatilidad). */
export function fmtShare(v: unknown, digits = 1): string {
	return fmtPct(v, { digits });
}

export function fmtScore(v: unknown): string {
	return isNum(v) ? fmtNumber(v, 1) : EMPTY;
}

export type ScoreSignal = 'positive' | 'mixed' | 'negative' | 'neutral';

/** Mismos cortes que las etiquetas del backend: ≥ 60 positivo, 45-60 mixto, < 45 negativo. */
export function scoreSignal(v: number | null | undefined): ScoreSignal {
	if (!isNum(v)) return 'neutral';
	if (v >= 60) return 'positive';
	if (v >= 45) return 'mixed';
	return 'negative';
}

const SCORE_CLASS: Record<ScoreSignal, string> = {
	positive: 'font-bold text-brand-green',
	mixed: 'font-bold text-brand-accent',
	negative: 'font-bold text-brand-red',
	neutral: 'text-brand-muted'
};

export function scoreClass(v: number | null | undefined): string {
	return SCORE_CLASS[scoreSignal(v)];
}

export const CATEGORY_LABELS: Record<keyof UsScoreCategories, string> = {
	valuation: 'Valoración',
	profitability: 'Rentabilidad',
	financial_health: 'Salud financiera',
	shareholder_return: 'Retorno al accionista',
	technical: 'Técnico'
};

export const CATEGORY_KEYS = Object.keys(CATEGORY_LABELS) as (keyof UsScoreCategories)[];

/** Plazos de menor a mayor (sin YTD, que se reinicia cada año). */
const GROWTH_KEYS = ['return_1d', 'return_5d', 'return_3m', 'return_6m', 'return_1y'] as const;

/** "Rendimiento creciente": 1D < 5D < 3M < 6M < 1A. */
export function isGrowingPerformance(
	row: Pick<UsCandidateRow, (typeof GROWTH_KEYS)[number]>
): boolean {
	for (let i = 1; i < GROWTH_KEYS.length; i++) {
		const prev = row[GROWTH_KEYS[i - 1]];
		const curr = row[GROWTH_KEYS[i]];
		if (!isNum(prev) || !isNum(curr) || curr <= prev) return false;
	}
	return true;
}

/** Fila plana con el 100 % de los datos de un candidato (para CSV). */
export function flattenCandidate(row: UsCandidateRow): Record<string, string | number | null> {
	const t = row.technical;
	const v = row.valuation;
	const p = row.profitability;
	const f = row.financial_health;
	const s = row.score;
	return {
		ticker: row.ticker,
		name: row.name ?? null,
		sector: row.sector ?? null,
		market_cap: row.market_cap ?? null,
		price: row.price,
		momentum_12_1: row.momentum_12_1,
		return_1d: row.return_1d ?? null,
		return_5d: row.return_5d ?? null,
		return_3m: row.return_3m ?? null,
		return_6m: row.return_6m ?? null,
		return_1y: row.return_1y ?? null,
		return_ytd: row.return_ytd ?? null,
		technical_sma_50: t?.sma_50 ?? null,
		technical_sma_200: t?.sma_200 ?? null,
		technical_rsi_14: t?.rsi_14 ?? null,
		technical_macd: t?.macd?.macd ?? null,
		technical_macd_signal: t?.macd?.signal ?? null,
		technical_macd_histogram: t?.macd?.histogram ?? null,
		technical_volume: t?.volume ?? null,
		technical_average_volume: t?.average_volume ?? null,
		valuation_per_trailing: v?.per_trailing ?? null,
		valuation_per_forward: v?.per_forward ?? null,
		valuation_peg_ratio: v?.peg_ratio ?? null,
		valuation_ev_to_ebitda: v?.ev_to_ebitda ?? null,
		valuation_price_to_book: v?.price_to_book ?? null,
		valuation_eps_trailing: v?.eps_trailing ?? null,
		valuation_eps_forward: v?.eps_forward ?? null,
		profitability_roe: p?.roe ?? null,
		profitability_roa: p?.roa ?? null,
		profitability_net_margin: p?.net_margin ?? null,
		financial_health_debt_to_equity: f?.debt_to_equity ?? null,
		financial_health_current_ratio: f?.current_ratio ?? null,
		financial_health_free_cash_flow: f?.free_cash_flow ?? null,
		shareholder_return_dividend_yield: row.shareholder_return?.dividend_yield ?? null,
		score_overall: s?.overall ?? null,
		score_label: s?.label ?? null,
		score_valuation: s?.categories?.valuation ?? null,
		score_profitability: s?.categories?.profitability ?? null,
		score_financial_health: s?.categories?.financial_health ?? null,
		score_shareholder_return: s?.categories?.shareholder_return ?? null,
		score_technical: s?.categories?.technical ?? null
	};
}

/** Columnas del CSV completo (41 campos, como el original). */
export function candidateCsvColumns<T extends UsCandidateRow>(): {
	key: string;
	label: string;
	csv: (value: unknown, row: T) => string | number | null;
}[] {
	const keys = Object.keys(
		flattenCandidate({ ticker: '', price: 0, momentum_12_1: 0 } as UsCandidateRow)
	);
	return keys.map((key) => ({
		key,
		label: key,
		csv: (_: unknown, row: T) => flattenCandidate(row)[key]
	}));
}
