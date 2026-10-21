<script lang="ts">
	import type { UsTickerEvaluation } from '$lib/api/types';
	import {
		ErrorState,
		KpiCard,
		KpiGrid,
		Notice,
		Section,
		SignalBadge,
		TickerLink
	} from '$lib/components/shared';
	import { fmtDate, fmtNumber } from '$lib/format';
	import { TickerSearch, UsScore } from '$lib/modules/us';
	import {
		CATEGORY_KEYS,
		CATEGORY_LABELS,
		fmtPrice,
		fmtScore,
		fmtShare,
		fmtUsd,
		scoreSignal
	} from '$lib/modules/us/format';

	let { data } = $props();

	const ev = $derived(data.evaluation.ok ? data.evaluation.data : null);

	type Dato = [label: string, value: string];
	function grupos(e: UsTickerEvaluation): { titulo: string; datos: Dato[] }[] {
		const t = e.technical;
		const x = (v: number | null | undefined, d = 2) => (v == null ? '—' : `${fmtNumber(v, d)}x`);
		return [
			{
				titulo: 'Valoración',
				datos: [
					['PER trailing', x(e.valuation.per_trailing)],
					['PER forward', x(e.valuation.per_forward)],
					['PEG', x(e.valuation.peg_ratio)],
					['EV/EBITDA', x(e.valuation.ev_to_ebitda)],
					['Precio/Libro', x(e.valuation.price_to_book)],
					['EPS trailing', fmtPrice(e.valuation.eps_trailing)],
					['EPS forward', fmtPrice(e.valuation.eps_forward)]
				]
			},
			{
				titulo: 'Rentabilidad',
				datos: [
					['ROE', fmtShare(e.profitability.roe)],
					['ROA', fmtShare(e.profitability.roa)],
					['Margen neto', fmtShare(e.profitability.net_margin)]
				]
			},
			{
				titulo: 'Salud financiera y dividendo',
				datos: [
					['Deuda/Patrimonio (%)', fmtNumber(e.financial_health.debt_to_equity, 1)],
					['Liquidez corriente', x(e.financial_health.current_ratio)],
					['Flujo de caja libre', fmtUsd(e.financial_health.free_cash_flow)],
					['Dividend yield', fmtShare(e.shareholder_return.dividend_yield, 2)]
				]
			},
			{
				titulo: 'Técnico',
				datos: [
					['SMA 50', fmtPrice(t.sma_50)],
					['SMA 200', fmtPrice(t.sma_200)],
					['RSI 14', fmtNumber(t.rsi_14, 1)],
					['MACD (histograma)', fmtNumber(t.macd?.histogram)],
					['Volumen', fmtUsd(t.volume).replace('$', '')],
					['Volumen prom. 30d', fmtUsd(t.average_volume).replace('$', '')]
				]
			}
		];
	}
</script>

<Section title="Evaluación en vivo · {data.ticker}">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<p class="max-w-3xl text-sm text-brand-muted">
			Datos de Yahoo Finance al momento de la consulta, con el mismo score 0-100 que la corrida
			batch. No depende de que el ticker haya pasado el filtro de market cap.
		</p>
		<TickerSearch value={data.ticker} />
	</div>

	{#if !ev}
		{#if !data.evaluation.ok}
			<ErrorState
				title="No se pudo evaluar {data.ticker}"
				message={data.evaluation.message}
				code={data.evaluation.code}
				retry={data.evaluation.code !== 'ticker_not_found'}
			/>
		{/if}
	{:else}
		<div>
			<p class="text-lg font-bold">
				{ev.name} <span class="font-normal text-brand-label">· {ev.sector || '—'}</span>
			</p>
			<p class="text-sm text-brand-muted">
				{ev.industry || '—'} · <TickerLink ticker={ev.ticker} /> en StockAnalysis · evaluado {fmtDate(
					ev.fetched_at,
					true
				)}{ev.cached ? ' (caché de 5 min)' : ''}
			</p>
		</div>
		<KpiGrid>
			<KpiCard label="Precio" value={fmtPrice(ev.price)} hint={ev.currency} />
			<KpiCard label="Market cap" value={fmtUsd(ev.market_cap)} />
			<KpiCard
				label="Score de salud"
				value={fmtScore(ev.score.overall)}
				tone={{ positive: 'green', mixed: 'accent', negative: 'red', neutral: 'text' }[
					scoreSignal(ev.score.overall)
				] as 'green' | 'accent' | 'red' | 'text'}
				hint={ev.score.label}
			/>
		</KpiGrid>
		<div>
			<SignalBadge signal={scoreSignal(ev.score.overall)} label={ev.score.label} />
		</div>
	{/if}
</Section>

{#if ev}
	<Section title="Score por categoría">
		<div class="grid max-w-3xl gap-2">
			{#each CATEGORY_KEYS as key (key)}
				<div
					class="flex items-center justify-between gap-4 rounded border border-brand-border bg-brand-card px-4 py-2"
				>
					<span class="text-sm text-brand-label">{CATEGORY_LABELS[key]}</span>
					<UsScore score={ev.score.categories[key]} />
				</div>
			{/each}
		</div>
	</Section>

	<Section title="Datos">
		<div class="grid grid-cols-1 gap-5 sm:grid-cols-2 xl:grid-cols-4">
			{#each grupos(ev) as grupo (grupo.titulo)}
				<div class="rounded border border-brand-border bg-brand-card p-4">
					<div class="mb-2 text-xs font-bold tracking-wide text-brand-label uppercase">
						{grupo.titulo}
					</div>
					<dl class="space-y-1">
						{#each grupo.datos as [label, value] (label)}
							<div class="flex justify-between gap-3 text-sm">
								<dt class="text-brand-muted">{label}</dt>
								<dd class="font-bold tabular-nums">{value}</dd>
							</div>
						{/each}
					</dl>
				</div>
			{/each}
		</div>
		<Notice kind="neutral">
			<p>Heurística de reglas simples para comparar rápido. No es asesoramiento financiero.</p>
		</Notice>
	</Section>
{/if}
