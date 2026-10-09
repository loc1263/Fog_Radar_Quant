<script lang="ts">
	import { ErrorState, Section } from '$lib/components/shared';
	import PortfolioWorkbench from './PortfolioWorkbench.svelte';

	let { data, form } = $props();
</script>

<Section title="Portafolio">
	<p class="max-w-3xl text-sm text-brand-muted">
		Modelo único en tres etapas: (1) ranking por score multi-factor, (2) diversificación por
		clustering jerárquico de la correlación, con un representante de cada cluster, y (3) pesos con
		Black-Litterman (equilibrio de mercado + views del score) y tope por activo, long-only.
	</p>
	{#if data.candidates.ok}
		<PortfolioWorkbench
			rows={data.candidates.data.rows}
			runId={data.candidates.data.run_id}
			limits={data.limits}
			{form}
		/>
	{:else}
		<ErrorState message={data.candidates.message} code={data.candidates.code} />
	{/if}
</Section>
