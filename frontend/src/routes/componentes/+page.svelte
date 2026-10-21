<!-- Vitrina de componentes (solo desarrollo): referencia visual del Dashboard ejecutivo. -->
<script lang="ts">
	import { BarChart, LineChart, ScatterChart, Sparkline } from '$lib/charts';
	import {
		CompositionBar,
		DataTable,
		EmptyState,
		KpiCard,
		KpiGrid,
		Notice,
		ScoreBar,
		Section,
		SignalBadge,
		TickerLink,
		WeightBar
	} from '$lib/components/shared';
	import { Button } from '$lib/components/ui/button';
	import { fmtPct } from '$lib/format';
	import type { Column } from '$lib/table';
	import { carteras, frontera, rows, sectores, series, type SampleRow } from './samples';

	const maxPeso = Math.max(...rows.map((r) => r.peso));
	const pct = (v: number) => fmtPct(v, { digits: 0 });

	const columns: Column<SampleRow>[] = [
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'nombre', label: 'Nombre', wrap: true },
		{ key: 'sector', label: 'Sector' },
		{ key: 'score', label: 'Score', numeric: true },
		{
			key: 'ret1y',
			label: 'Ret. 1 año',
			numeric: true,
			signed: true,
			format: (v) => fmtPct(v, { signed: true })
		},
		{ key: 'peso', label: 'Peso', numeric: true },
		{ key: 'precios', label: 'Precio 30d', sortable: false, csv: () => '' }
	];
</script>

<Notice kind="warning" title="Vitrina de componentes">
	<p>Datos ficticios. Esta página solo existe en desarrollo (<code>/componentes</code>).</p>
</Notice>

<Section title="Tarjetas KPI">
	<KpiGrid>
		<KpiCard label="Retorno esperado anual" value="18,4 %" tone="green" />
		<KpiCard label="Volatilidad anual" value="21,7 %" tone="blue" />
		<KpiCard label="Sharpe" value="0,85" tone="accent" />
		<KpiCard label="Retorno 1 mes" value="-3,2 %" signedValue={-0.032} />
		<KpiCard label="Universo" value="30 acciones" hint="IPSA, sin duplicados" />
	</KpiGrid>
</Section>

<Section title="Tabla (orden, búsqueda y CSV)">
	<DataTable {rows} {columns} csvFilename="vitrina" initialSort={{ key: 'score', dir: 'desc' }}>
		{#snippet cell({ column, row, text })}
			{#if column.key === 'ticker'}
				<TickerLink ticker={row.ticker} />
			{:else if column.key === 'score'}
				<ScoreBar score={row.score} />
			{:else if column.key === 'peso'}
				<WeightBar value={row.peso} max={maxPeso} label={fmtPct(row.peso)} />
			{:else if column.key === 'precios'}
				<Sparkline values={row.precios} />
			{:else}
				{text}
			{/if}
		{/snippet}
	</DataTable>
</Section>

<Section title="Badges, botones y mensajes">
	<div class="flex flex-wrap items-center gap-2">
		<SignalBadge signal="positive" label="Alcista" />
		<SignalBadge signal="negative" label="Bajista" />
		<SignalBadge signal="mixed" label="Lateral" />
		<SignalBadge signal="neutral" label="Sin datos" />
		<SignalBadge signal="category" label="Large cap" />
		<Button>Botón secundario</Button>
		<Button variant="primary">Primario</Button>
	</div>
	<Notice kind="info" title="Nota">
		<p>Caja informativa para contexto metodológico.</p>
	</Notice>
	<EmptyState>No hay corridas registradas todavía.</EmptyState>
</Section>

<Section title="Composición">
	<CompositionBar segments={sectores.map((s) => ({ label: s.sector, value: s.peso }))} />
</Section>

<Section title="Gráfico de líneas">
	<LineChart
		data={series}
		x="fecha"
		series={[
			{ key: 'portafolio', label: 'Portafolio' },
			{ key: 'benchmark', label: 'Benchmark' }
		]}
	/>
</Section>

<Section title="Barras">
	<BarChart
		data={sectores}
		category="sector"
		series={[{ key: 'peso', label: 'Peso' }]}
		valueFormat={pct}
	/>
</Section>

<Section title="Dispersión (frontera eficiente)">
	<ScatterChart
		x="vol"
		y="ret"
		xFormat={pct}
		yFormat={pct}
		groups={[
			{ key: 'frontera', label: 'Frontera eficiente', data: frontera },
			{ key: 'carteras', label: 'Carteras', data: carteras }
		]}
	/>
</Section>

<Section title="Sin datos">
	<LineChart data={[]} x="fecha" series={[{ key: 'a', label: 'A' }]} />
</Section>
