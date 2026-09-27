<!-- Detalle desplegable de una acción del ranking: métricas crudas por grupo. -->
<script lang="ts">
	import type { ClRankingRow } from '$lib/api/types';
	import type { ClUnidad } from '$lib/api/types';
	import { fmtValor } from './format';

	let { fila }: { fila: ClRankingRow } = $props();

	type Dato = [label: string, unidad: ClUnidad, valor: number | null];

	const grupos = $derived.by((): { titulo: string; datos: Dato[] }[] => {
		const m = fila.metricas;
		return [
			{
				titulo: 'Valoración',
				datos: [
					['P/E (ttm)', 'multiplo', m.trailing_pe],
					['Precio/Libro', 'multiplo', m.price_book],
					['Precio/Ventas', 'multiplo', m.price_sales],
					['EV/EBITDA', 'multiplo', m.ev_ebitda],
					['EV/Ventas', 'multiplo', m.ev_revenue]
				]
			},
			{
				titulo: 'Calidad y crecimiento',
				datos: [
					['Margen neto', 'pct', m.profit_margin],
					['Margen operacional', 'pct', m.operating_margin],
					['ROA', 'pct', m.roa],
					['ROE', 'pct', m.roe],
					['Crec. ingresos (a/a)', 'pct', m.revenue_growth_q],
					['Crec. utilidades (a/a)', 'pct', m.earnings_growth_q]
				]
			},
			{
				titulo: 'Salud financiera y dividendo',
				datos: [
					['Deuda/Patrimonio (%)', 'pct_pts', m.debt_equity],
					['Liquidez corriente', 'multiplo', m.current_ratio],
					['Dividend yield', 'pct_pts', m.dividend_yield]
				]
			},
			{
				titulo: 'Momentum (retorno de precio)',
				datos: [
					['5 días', 'pct', m.ret_5d],
					['1 mes', 'pct', m.ret_1m],
					['3 meses', 'pct', m.ret_3m],
					['6 meses', 'pct', m.ret_6m],
					['YTD', 'pct', m.ret_ytd],
					['1 año', 'pct', m.ret_1a]
				]
			}
		];
	});
</script>

<div class="grid grid-cols-1 gap-5 sm:grid-cols-2 xl:grid-cols-4">
	{#each grupos as grupo (grupo.titulo)}
		<div>
			<div class="mb-1.5 text-xs font-bold tracking-wide text-brand-label uppercase">
				{grupo.titulo}
			</div>
			<dl class="space-y-1">
				{#each grupo.datos as [label, unidad, valor] (label)}
					<div class="flex justify-between gap-3 text-sm">
						<dt class="text-brand-muted">{label}</dt>
						<dd class="font-bold tabular-nums">{fmtValor(unidad, valor)}</dd>
					</div>
				{/each}
			</dl>
		</div>
	{/each}
</div>
<p class="mt-3 text-xs text-brand-muted">
	{fila.industria ?? 'Industria sin dato'} · {fila.metricas_disponibles} de {fila.metricas_totales}
	métricas con dato. Los múltiplos ≤ 0 y el dividendo con payout &gt; 100 % no puntúan.
</p>
