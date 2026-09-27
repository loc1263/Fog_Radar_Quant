<script lang="ts">
	import { ErrorState, Section } from '$lib/components/shared';
	import PortfolioWorkbench from './PortfolioWorkbench.svelte';

	let { data, form } = $props();
</script>

<Section title="Portafolio · {data.method.label}">
	<p class="max-w-3xl text-sm text-brand-muted">{data.method.description}</p>
	{#if data.candidates.ok}
		<!-- Un banco de trabajo por método: selección y resultados no se mezclan entre pestañas. -->
		{#key data.method.slug}
			<PortfolioWorkbench
				method={data.method}
				rows={data.candidates.data.rows}
				runId={data.candidates.data.run_id}
				limits={data.limits}
				{form}
			/>
		{/key}
	{:else}
		<ErrorState message={data.candidates.message} code={data.candidates.code} />
	{/if}
</Section>
