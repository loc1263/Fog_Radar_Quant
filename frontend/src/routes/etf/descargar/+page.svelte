<script lang="ts">
	import { asset } from '$app/paths';
	import { ErrorState, Section } from '$lib/components/shared';
	import { fmtFechaCorrida } from '$lib/modules/etf/format';

	let { data } = $props();

	const resumen = $derived(data.resumen.ok ? data.resumen.data : null);

	const SECCIONES = [
		'Top 15 recomendados',
		'Excluidos por distorsión estadística',
		'Rotación vs. corrida anterior',
		'Diversificación del Top 15',
		'Ranking por categoría',
		'Top 15 Momentum / crecimiento',
		'Top 15 (rendimiento + Beta)',
		'Top 15 Cap.'
	];
</script>

<Section title="Descargar informe">
	{#if !resumen}
		{#if !data.resumen.ok}
			<ErrorState message={data.resumen.message} code={data.resumen.code} />
		{/if}
	{:else}
		<div class="rounded border border-brand-border bg-brand-card p-4">
			<div class="font-bold text-brand-label">Informe ejecutivo en PDF</div>
			<p class="mt-2 text-sm">
				PDF de la corrida vigente ({fmtFechaCorrida(resumen.fecha)}), Letter apaisado, con las
				mismas secciones que este panel:
			</p>
			<ul class="mt-3 grid grid-cols-1 gap-1.5 text-sm sm:grid-cols-2">
				{#each SECCIONES as s (s)}
					<li class="flex items-center gap-2">
						<span class="h-1.5 w-1.5 shrink-0 rounded-full bg-brand-blue"></span>{s}
					</li>
				{/each}
			</ul>
			<a
				href={asset('/data/api/etf/pdf.pdf')}
				download="etf_informe.pdf"
				class="mt-4 inline-block rounded border border-brand-strong bg-brand-strong px-3 py-1.5 text-sm font-bold text-white hover:opacity-90"
				>Descargar PDF</a
			>
		</div>
	{/if}
</Section>
