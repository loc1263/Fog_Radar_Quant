<script lang="ts">
	import { resolve } from '$app/paths';
	import type { TdArchivo } from '$lib/api/types';
	import { DataTable, Section } from '$lib/components/shared';
	import { fmtNumber } from '$lib/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	const d = $derived(data.latest.ok ? data.latest.data : null);
	const archivos = $derived(d?.archivos ?? []);
	const pdf = $derived(archivos.find((f) => f.nombre.endsWith('.pdf')));

	const TIPOS: Record<string, string> = {
		'application/pdf': 'PDF',
		'text/csv; charset=utf-8': 'CSV',
		'application/json': 'JSON',
		'text/plain; charset=utf-8': 'Texto'
	};
	const columns: Column<TdArchivo>[] = [
		{ key: 'nombre', label: 'Archivo' },
		{ key: 'tipo', label: 'Tipo', format: (v) => TIPOS[String(v)] ?? String(v) },
		{
			key: 'bytes',
			label: 'Tamaño',
			numeric: true,
			format: (v) => `${fmtNumber(Number(v) / 1024, 0)} KB`
		}
	];
</script>

{#if d}
	<Section title="Descargar informe y archivos">
		{#if pdf}
			<div class="rounded border border-brand-border bg-brand-card p-4">
				<div class="font-bold text-brand-label">Informe de portafolio en PDF</div>
				<p class="mt-2 text-sm">
					Informe generado por el pipeline para la corrida {d.run_dir}: resumen, portafolio,
					proyección, sectores y momentum de cada análisis.
				</p>
				<a
					href={resolve('/tendencia/descargar-archivo/[run]/[archivo]', {
						run: d.run_dir,
						archivo: pdf.nombre
					})}
					download
					data-sveltekit-reload
					class="mt-4 inline-block rounded border border-brand-strong bg-brand-strong px-3 py-1.5 text-sm font-bold text-white hover:opacity-90"
					>Descargar PDF</a
				>
			</div>
		{/if}
		<DataTable rows={archivos} {columns} caption="Archivos de la corrida">
			{#snippet cell({ column, row, text })}
				{#if column.key === 'nombre'}
					<a
						href={resolve('/tendencia/descargar-archivo/[run]/[archivo]', {
							run: d.run_dir,
							archivo: row.nombre
						})}
						download
						data-sveltekit-reload
						class="font-bold text-brand-label hover:text-brand-accent hover:underline"
						>{row.nombre}</a
					>
				{:else}
					{text}
				{/if}
			{/snippet}
		</DataTable>
	</Section>
{/if}
