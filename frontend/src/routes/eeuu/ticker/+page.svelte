<script lang="ts">
	import { Notice, Section } from '$lib/components/shared';
	import { TickerSearch } from '$lib/modules/us';

	let { data } = $props();
</script>

<Section title="Evaluación en vivo">
	<p class="max-w-3xl text-sm text-brand-muted">
		Consulta Yahoo Finance al momento para cualquier ticker de EEUU y calcula el mismo score de
		salud 0-100 que la corrida batch, aunque no haya pasado el filtro de market cap.
	</p>
	<TickerSearch value={data.invalid ?? ''} />
	{#if data.invalid !== null}
		<Notice kind="warning"><p>"{data.invalid}" no es un ticker válido.</p></Notice>
	{/if}
</Section>
