<script lang="ts">
	import { resolve } from '$app/paths';
	import type { ResultadoBusqueda } from '$lib/api/types';
	import { DataTable, EmptyState, ErrorState, Section } from '$lib/components/shared';
	import { destinoResultado, fmtPosicion, fmtValor } from '$lib/transversal';
	import type { Column } from '$lib/table';

	let { data } = $props();

	type Fila = ResultadoBusqueda & { valor_txt: string; posicion_txt: string };

	const filas = $derived<Fila[]>(
		data.resultados?.ok
			? data.resultados.data.resultados.map((r) => ({
					...r,
					valor_txt: fmtValor(r.valor, r.unidad),
					posicion_txt: fmtPosicion(r.posicion, r.total)
				}))
			: []
	);
	const modulos = $derived(new Set(filas.map((f) => f.modulo)).size);

	const columns: Column<Fila>[] = [
		{ key: 'titulo_modulo', label: 'Módulo' },
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'nombre', label: 'Nombre', wrap: true },
		{ key: 'contexto', label: 'Dónde aparece', wrap: true },
		{ key: 'posicion', label: 'Posición', numeric: true, format: (_v, r) => r.posicion_txt },
		{ key: 'metrica', label: 'Métrica' },
		{ key: 'valor', label: 'Valor', numeric: true, format: (_v, r) => r.valor_txt }
	];
</script>

<svelte:head><title>Buscar · Fog Radar Quant</title></svelte:head>

<Section title="Buscar un ticker en los 4 módulos">
	<form method="GET" class="flex flex-wrap items-end gap-2" role="search">
		<label class="block">
			<span class="mb-1 block text-sm font-bold text-brand-label">Ticker o nombre</span>
			<input
				type="search"
				name="q"
				value={data.q}
				required
				maxlength="40"
				placeholder="p. ej. NVDA, SQM-B, Samsung"
				class="w-72 rounded border border-brand-border bg-brand-bg px-2 py-1.5 text-sm"
			/>
		</label>
		<button
			type="submit"
			class="rounded bg-brand-strong px-3 py-1.5 text-sm font-bold text-white hover:opacity-90"
		>
			Buscar
		</button>
	</form>
	<p class="mt-2 text-sm text-brand-label">
		Busca por ticker (con o sin sufijo de bolsa: <code>SQM-B</code> encuentra
		<code>SQM-B.SN</code>) o por parte del nombre, en la última corrida de cada módulo.
	</p>
</Section>

{#if data.resultados}
	<Section title="Resultados para “{data.q}”">
		{#if !data.resultados.ok}
			<ErrorState
				title="No se pudo buscar"
				message={data.resultados.message}
				code={data.resultados.code}
			/>
		{:else if filas.length === 0}
			<EmptyState>Ningún módulo tiene ese ticker o nombre en su última corrida.</EmptyState>
		{:else}
			<p class="mb-2 text-sm">
				{filas.length} resultado{filas.length === 1 ? '' : 's'} en {modulos} módulo{modulos === 1
					? ''
					: 's'}.
			</p>
			<DataTable rows={filas} {columns} csvFilename="busqueda_{data.q}">
				{#snippet cell({ column, row, text })}
					{#if column.key === 'ticker'}
						{@const destino = destinoResultado(row)}
						{#if destino}
							<a
								href="{resolve(destino.path)}{destino.query}"
								class="font-bold text-brand-label underline hover:text-brand-accent">{row.ticker}</a
							>
						{:else}
							<span class="font-bold">{row.ticker}</span>
						{/if}
						{#if row.exacto}<span class="ml-1 text-xs text-brand-accent">exacto</span>{/if}
					{:else}
						{text}
					{/if}
				{/snippet}
			</DataTable>
		{/if}
	</Section>
{/if}
