/**
 * Lógica común de las vistas de historial (history.db de cada módulo, ver
 * `backend/src/fog_radar/core/histdb.py`): elegir corridas/ticker desde la URL y
 * resumir un diff entre dos corridas.
 */
import type { HistDiffFila, HistEstado } from '$lib/api/types';

export const ESTADOS: Record<
	HistEstado,
	{ label: string; signal: 'positive' | 'negative' | 'neutral' }
> = {
	nuevo: { label: 'Entró', signal: 'positive' },
	salio: { label: 'Salió', signal: 'negative' },
	sigue: { label: 'Se mantiene', signal: 'neutral' },
	fuera: { label: 'Fuera', signal: 'neutral' }
};

export interface ResumenDiff {
	nuevos: number;
	salidas: number;
	siguen: number;
	/** Entre los que se mantienen: cuántos subieron / bajaron de puesto. */
	suben: number;
	bajan: number;
}

export function resumenDiff(
	filas: readonly Pick<HistDiffFila, 'estado' | 'delta_rank'>[]
): ResumenDiff {
	const r: ResumenDiff = { nuevos: 0, salidas: 0, siguen: 0, suben: 0, bajan: 0 };
	for (const f of filas) {
		if (f.estado === 'nuevo') r.nuevos++;
		else if (f.estado === 'salio') r.salidas++;
		else if (f.estado === 'sigue') {
			r.siguen++;
			if ((f.delta_rank ?? 0) > 0) r.suben++;
			else if ((f.delta_rank ?? 0) < 0) r.bajan++;
		}
	}
	return r;
}

/**
 * Valor de un parámetro de la URL si está entre las opciones válidas; si no, el
 * valor por defecto. Evita mandar al backend cualquier cosa que venga en la URL.
 */
export function elegir<T extends string>(
	pedido: string | null,
	opciones: readonly T[],
	porDefecto: T | null
): T | null {
	return pedido !== null && (opciones as readonly string[]).includes(pedido)
		? (pedido as T)
		: porDefecto;
}

/**
 * Las dos corridas a comparar: las de la URL si existen y son distintas, si no
 * la penúltima y la última. `null` con menos de dos corridas.
 */
export function parDeCorridas<T extends string>(
	claves: readonly T[],
	desde: string | null,
	hasta: string | null
): { desde: T; hasta: T } | null {
	if (claves.length < 2) return null;
	const d = elegir(desde, claves, null);
	const h = elegir(hasta, claves, null);
	if (d && h && d !== h) return { desde: d, hasta: h };
	return { desde: claves[claves.length - 2], hasta: claves[claves.length - 1] };
}

/** `20260929_193618` → `29-09-2026 19:36` (timestamp de corrida del módulo Chile). */
export function fmtTs(ts: string): string {
	const m = /^(\d{4})(\d{2})(\d{2})_(\d{2})(\d{2})/.exec(ts);
	return m ? `${m[3]}-${m[2]}-${m[1]} ${m[4]}:${m[5]}` : ts;
}

/** `2026-09-29T22-39-31Z` → `29-09-2026 22:39 UTC` (run_id del módulo EEUU). */
export function fmtRunId(runId: string): string {
	const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2})-(\d{2})/.exec(runId);
	return m ? `${m[3]}-${m[2]}-${m[1]} ${m[4]}:${m[5]} UTC` : runId;
}

/** `20260917_2` → `17-09-2026 (2)` (clave de corrida del módulo ETF). */
export function fmtRunKey(key: string): string {
	const m = /^(\d{4})(\d{2})(\d{2})(?:_(\d+))?$/.exec(key);
	if (!m) return key;
	return `${m[3]}-${m[2]}-${m[1]}${m[4] ? ` (${m[4]})` : ''}`;
}
