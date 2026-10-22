<script lang="ts">
	import { page } from '$app/state';
	import type { TdHolding } from '$lib/api/types';
	import { DataTable, Section, TickerLink } from '$lib/components/shared';
	import { Senal } from '$lib/modules/td';
	import { elegirAnalisis, money, pctPts, sufijoAnalisis } from '$lib/modules/td/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const d = $derived(data.latest.ok ? data.latest.data : null);
	const a = $derived(d ? elegirAnalisis(d, page.url.searchParams.get('analisis')) : null);
	const p = $derived(d?.parametros);

	const columns: Column<TdHolding>[] = [
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'nombre', label: 'Empresa', wrap: true },
		{ key: 'precio_actual_usd', label: 'Precio actual', numeric: true, format: (v) => money(v) },
		{
			key: 'precio_proyectado_6m_usd',
			label: 'Proyectado 6m',
			numeric: true,
			format: (v) => money(v)
		},
		{
			key: 'precio_min_90pct_6m_usd',
			label: 'Rango 90 %',
			numeric: true,
			format: (_v, r) => `${money(r.precio_min_90pct_6m_usd)} – ${money(r.precio_max_90pct_6m_usd)}`
		},
		{
			key: 'retorno_esperado_6m_pct',
			label: 'Retorno 6m',
			numeric: true,
			signed: true,
			format: (v) => pctPts(v, 1, true)
		},
		{ key: 'senal_tecnica_3m', label: 'Señal 3m' },
		{ key: 'senal_tecnica', label: 'Señal 6m' }
	];
</script>

{#if a}
	<Section title="Proyección de tendencia a 6 meses{sufijoAnalisis(a)}">
		<p class="text-xs text-brand-muted">
			Proyección GBM (movimiento geométrico browniano) con el retorno y la volatilidad anualizados
			de cada acción: el precio central es el escenario más probable y el rango, una banda de
			confianza aprox. del 90 %. <strong>No es una predicción garantizada</strong>; se basa en los
			últimos 5 años. Las señales técnicas son solo informativas (no afectan proyección ni pesos):
			la Señal 6m cruza medias móviles de {p?.ma_corta ?? 10} y {p?.ma_larga ?? 40} semanas; la Señal
			3m, de {p?.ma_corta_3m ?? 4} y {p?.ma_larga_3m ?? 13}, y reacciona antes.
		</p>
		<DataTable
			rows={a.holdings}
			{columns}
			initialSort={{ key: 'retorno_esperado_6m_pct', dir: 'desc' }}
			csvFilename="tendencia_proyeccion6m{a.key}_{d?.run_dir}"
			caption="Proyección a 6 meses"
		>
			{#snippet cell({ column, row, text })}
				{#if column.key === 'ticker'}
					<TickerLink ticker={row.ticker} />
				{:else if column.key === 'senal_tecnica' || column.key === 'senal_tecnica_3m'}
					<Senal valor={row[column.key]} />
				{:else}
					{text}
				{/if}
			{/snippet}
		</DataTable>
	</Section>
{/if}
