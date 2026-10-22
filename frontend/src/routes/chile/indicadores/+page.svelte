<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { ClColumnaInfo } from '$lib/api/types';
	import { DataTable, ErrorState, Notice, Section, TickerLink } from '$lib/components/shared';
	import { fmtDate } from '$lib/format';
	import { esNumerica, fmtValor } from '$lib/modules/cl/format';
	import type { Column } from '$lib/table';
	import { cn } from '$lib/utils';

	let { data } = $props();

	const ind = $derived(data.indicadores.ok ? data.indicadores.data : null);

	const TODOS = 'Todos';
	// El grupo va en la URL (?grupo=…): enlace compartible y funciona sin JavaScript.
	const grupoElegido = $derived(page.url.searchParams.get('grupo'));
	// Por defecto, el primer grupo con datos de mercado (Valoración).
	const grupo = $derived(
		grupoElegido && (grupoElegido === TODOS || ind?.grupos.includes(grupoElegido))
			? grupoElegido
			: (ind?.grupos.find((g) => g !== 'Identificación') ?? TODOS)
	);

	type Fila = { simbolo: string; nombre: string | null; sector: string | null } & Record<
		string,
		unknown
	>;

	const filas = $derived<Fila[]>(
		(ind?.filas ?? []).map((f) => ({
			simbolo: f.simbolo,
			nombre: f.nombre,
			sector: f.sector,
			industria: f.industria,
			...f.valores
		}))
	);

	const visibles = $derived<ClColumnaInfo[]>(
		(ind?.columnas ?? []).filter((c) => grupo === TODOS || c.grupo === grupo)
	);

	const columns = $derived<Column<Fila>[]>([
		{ key: 'simbolo', label: 'Acción' },
		{ key: 'nombre', label: 'Nombre', wrap: true },
		{ key: 'sector', label: 'Sector', wrap: true },
		...visibles.map((c): Column<Fila> => ({
			key: c.key,
			label: c.label,
			numeric: esNumerica(c.unidad),
			format: (v) => fmtValor(c.unidad, v),
			title: c.descripcion ?? undefined
		}))
	]);

	const csvName = $derived(
		`indicadores_ipsa_${grupo
			.normalize('NFD')
			.replace(/\p{Diacritic}/gu, '')
			.toLowerCase()
			.replace(/\s+/g, '_')}`
	);
</script>

{#if !ind}
	{#if !data.indicadores.ok}
		<ErrorState
			title={data.indicadores.code === 'no_data' ? 'Aún no hay indicadores del IPSA' : undefined}
			message={data.indicadores.message}
			code={data.indicadores.code}
		/>
	{/if}
{:else}
	<Section title="Indicadores financieros">
		<p class="text-xs text-brand-muted">
			Datos crudos de Yahoo Finance antes del scoring ({ind.filas.length} acciones, {ind.columnas
				.length} indicadores) · {fmtDate(ind.generado_en, true)} · {ind.generado_desde}
		</p>
		<nav class="flex flex-wrap gap-1.5" aria-label="Grupo de indicadores">
			{#each [...ind.grupos, TODOS] as g (g)}
				<a
					href="{resolve('/chile/indicadores')}?grupo={encodeURIComponent(g)}"
					data-sveltekit-noscroll
					data-sveltekit-replacestate
					aria-current={grupo === g ? 'true' : undefined}
					class={cn(
						'rounded border px-2.5 py-0.5 text-xs font-bold',
						grupo === g
							? 'border-brand-strong bg-brand-strong text-white'
							: 'border-brand-border bg-brand-bg text-brand-label hover:bg-brand-card'
					)}>{g}</a
				>
			{/each}
		</nav>
		<DataTable
			rows={filas}
			{columns}
			csvFilename={csvName}
			caption="Indicadores financieros del IPSA"
		>
			{#snippet cell({ column, row, text })}
				{#if column.key === 'simbolo'}
					<TickerLink ticker={row.simbolo} />
				{:else}
					{text}
				{/if}
			{/snippet}
		</DataTable>
		<Notice size="compact" kind="info" title="Cómo leer los datos">
			<p>
				Montos en la moneda de cotización (CLP). Si Yahoo reporta en otra moneda (p. ej. USD), se
				convierten con la tasa del grupo Identificación y los múltiplos EV/EBITDA, EV/Ventas,
				Precio/Ventas y Precio/Libro se recalculan.
			</p>
		</Notice>
	</Section>
{/if}
