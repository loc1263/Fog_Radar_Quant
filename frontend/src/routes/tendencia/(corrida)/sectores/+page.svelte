<script lang="ts">
	import { page } from '$app/state';
	import type { TdSector } from '$lib/api/types';
	import { BarChart } from '$lib/charts';
	import { DataTable, Section, TickerLink } from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import { Senal } from '$lib/modules/td';
	import {
		elegirAnalisis,
		pct,
		pctPts,
		separarTickers,
		sufijoAnalisis
	} from '$lib/modules/td/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const d = $derived(data.latest.ok ? data.latest.data : null);
	const a = $derived(d ? elegirAnalisis(d, page.url.searchParams.get('analisis')) : null);
	const grafico = $derived(
		(a?.sectors ?? [])
			.map((s) => ({ sector: s.sector, peso: s.peso_total * 100 }))
			.sort((x, y) => y.peso - x.peso)
	);

	const columns: Column<TdSector>[] = [
		{ key: 'sector', label: 'Sector', wrap: true },
		{ key: 'n_acciones', label: 'N° acciones', numeric: true },
		{ key: 'peso_total', label: 'Peso total', numeric: true, format: (v) => pct(v) },
		{
			key: 'retorno_6m_prom',
			label: 'Retorno 6m prom.',
			numeric: true,
			signed: true,
			format: (v) => pctPts(v)
		},
		{ key: 'senal_predominante', label: 'Señal predominante' },
		{ key: 'tickers', label: 'Tickers', wrap: true, sortable: false }
	];
</script>

{#if a}
	<Section title="Sector en tendencia{sufijoAnalisis(a)}">
		<BarChart
			data={grafico}
			category="sector"
			series={[{ key: 'peso', label: 'Peso en el portafolio', color: 'var(--color-brand-blue)' }]}
			labelWidth={170}
			height={Math.max(200, grafico.length * 34)}
			valueFormat={(v) => `${fmtNumber(v, 0)} %`}
		/>
		<DataTable
			rows={a.sectors}
			{columns}
			initialSort={{ key: 'peso_total', dir: 'desc' }}
			csvFilename="tendencia_sectores{a.key}_{d?.run_dir}"
			caption="Sectores del portafolio"
		>
			{#snippet cell({ column, row, text })}
				{#if column.key === 'senal_predominante'}
					<Senal valor={row.senal_predominante} />
				{:else if column.key === 'tickers'}
					{#each separarTickers(row.tickers) as t, i (t)}{#if i},
						{/if}<TickerLink ticker={t} />{/each}
				{:else if column.key === 'sector'}
					<span class="font-bold">{text}</span>
				{:else}
					{text}
				{/if}
			{/snippet}
		</DataTable>
	</Section>
{/if}
