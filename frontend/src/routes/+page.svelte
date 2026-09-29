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
	<Notice kind="info" title="Migración completa">
		<p>
			Los cuatro proyectos originales (Chile, EEUU, ETFs y Tendencia) ya funcionan en este sitio,
			con paridad verificada contra cada original. Siguen la consolidación (cliente de Yahoo común,
			un comando para actualizar todo) y el despliegue en producción.
		</p>
	</Notice>
</Section>
