<script lang="ts">
	import { ErrorState, ModuleCard, Notice, Section } from '$lib/components/shared';
	import { MODULES } from '$lib/nav';

	let { data } = $props();

	const byKey = $derived(
		data.status.ok ? Object.fromEntries(data.status.data.modules.map((m) => [m.key, m])) : {}
	);
</script>

<Section title="Módulos">
	{#if !data.status.ok}
		<ErrorState
			title="Backend no disponible"
			message={data.status.message}
			code={data.status.code}
		/>
	{/if}
	<div class="flex flex-wrap gap-3">
		{#each MODULES as nav (nav.key)}
			<ModuleCard {nav} status={byKey[nav.key]} />
		{/each}
	</div>
</Section>

<Section title="Estado del proyecto">
	<Notice kind="info" title="Sitio en construcción">
		<p>
			Los cuatro proyectos originales se están migrando a este sitio, un módulo a la vez: Chile →
			EEUU → ETFs → Tendencia. Mientras tanto, cada módulo muestra su estado de migración.
		</p>
	</Notice>
</Section>
