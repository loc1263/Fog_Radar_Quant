<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { TdHolding } from '$lib/api/types';
	import {
		CompositionBar,
		DataTable,
		Section,
		TickerLink,
		WeightBar
	} from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { Senal } from '$lib/modules/td';
	import {
		elegirAnalisis,
		elegirPortafolio,
		pct,
		pctPts,
		pesoPorPais,
		sufijoAnalisis
	} from '$lib/modules/td/format';
	import type { Column } from '$lib/table';
	import { cn } from '$lib/utils';

	let { data } = $props();

	const d = $derived(data.latest.ok ? data.latest.data : null);
	const a = $derived(d ? elegirAnalisis(d, page.url.searchParams.get('analisis')) : null);
	const nPedido = $derived(Number(page.url.searchParams.get('n')) || null);
	const p = $derived(a ? elegirPortafolio(a, nPedido) : null);
	const tamanos = $derived(a ? [...a.portfolio_options.map((o) => o.n), a.n_selected] : []);
	const maxPeso = $derived(Math.max(1e-9, ...(p?.holdings ?? []).map((h) => h.peso_final)));

	function hrefTamano(n: number): string {
		const params = new URL(page.url).searchParams;
		params.set('n', String(n));
		return `?${params}`;
	}

	const columns: Column<TdHolding>[] = [
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'nombre', label: 'Empresa', wrap: true },
		{ key: 'pais', label: 'País' },
		{ key: 'sector', label: 'Sector', wrap: true },
		{ key: 'peso_final', label: 'Peso en portafolio', numeric: true, format: (v) => pct(v) },
		{
			key: 'retorno_esperado_anual_pct',
			label: 'Retorno anual esp.',
			numeric: true,
			format: (v) => pctPts(v)
		},
		{ key: 'senal_tecnica_3m', label: 'Señal 3m' },
		{ key: 'senal_tecnica', label: 'Señal 6m' }
	];
</script>

{#if a && p}
	<Section title="Portafolio seleccionado{sufijoAnalisis(a)} — {p.holdings.length} acciones">
		{#if a.portfolio_options.length > 0}
			<div class="flex flex-wrap items-center gap-x-4 gap-y-2">
				<div
					class="inline-flex overflow-hidden rounded border border-brand-border text-sm"
					role="group"
					aria-label="Tamaño del portafolio"
				>
					{#each tamanos as n (n)}
						<a
							href="{resolve('/tendencia/portafolio')}{hrefTamano(n)}"
							data-sveltekit-noscroll
							aria-current={p.n === n ? 'true' : undefined}
							class={cn(
								'px-3 py-1.5 font-num',
								p.n === n
									? 'bg-brand-strong text-white'
									: 'bg-brand-bg text-brand-label hover:bg-brand-card'
							)}>{n} acciones</a
						>
					{/each}
				</div>
				<p class="text-xs text-brand-muted">
					Retorno anual esp. <b class="text-brand-text">{pct(p.retorno)}</b> · Volatilidad
					<b class="text-brand-text">{pct(p.volatilidad)}</b> · Sharpe
					<b class="text-brand-text">{fmtNumber(p.sharpe, 2)}</b>
				</p>
			</div>
			{#if !p.completo}
				<p class="text-xs text-brand-muted">
					Las {p.n} acciones de mayor peso del universo, con Markowitz (máx. Sharpe) corrido de nuevo
					solo sobre ellas: el 100 % se reparte entre las {p.n}.
				</p>
			{/if}
		{/if}
		<div>
			<div class="mb-2 text-sm font-bold text-brand-label">Asignación por país</div>
			<CompositionBar segments={pesoPorPais(p.holdings)} />
		</div>
		<DataTable
			rows={p.holdings}
			{columns}
			initialSort={{ key: 'peso_final', dir: 'desc' }}
			csvFilename="tendencia_portafolio{a.key}_{p.n}_{d?.run_dir}"
			caption="Portafolio seleccionado"
		>
			{#snippet cell({ column, row, text })}
				{#if column.key === 'ticker'}
					<TickerLink ticker={row.ticker} />
				{:else if column.key === 'peso_final'}
					<WeightBar value={row.peso_final} max={maxPeso} label={text} />
				{:else if column.key === 'senal_tecnica' || column.key === 'senal_tecnica_3m'}
					<Senal valor={row[column.key]} />
				{:else}
					{text}
				{/if}
			{/snippet}
		</DataTable>
	</Section>
{/if}
