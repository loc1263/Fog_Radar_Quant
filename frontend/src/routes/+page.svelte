<script lang="ts">
	import { resolve } from '$app/paths';
	import { ErrorState, ModuleCard, Notice, Section } from '$lib/components/shared';
	import { MODULES } from '$lib/nav';
	import { fmtDate } from '$lib/format';

	let { data } = $props();

	const byKey = $derived(
		data.hub.ok ? Object.fromEntries(data.hub.data.modulos.map((m) => [m.key, m])) : {}
	);
	const atrasados = $derived(
		data.hub.ok ? data.hub.data.modulos.filter((m) => m.frescura !== 'al_dia') : []
	);
</script>

<Section title="Buscar un ticker en los 4 módulos">
	<form
		method="GET"
		action={resolve('/buscar')}
		class="flex flex-wrap items-end gap-2"
		role="search"
	>
		<label class="block">
			<span class="mb-1 block text-sm font-bold text-brand-label">Ticker o nombre</span>
			<input
				type="search"
				name="q"
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
</Section>

<Section title="Módulos">
	{#if !data.hub.ok}
		<ErrorState title="Backend no disponible" message={data.hub.message} code={data.hub.code} />
	{:else}
		<p class="mb-3 text-sm text-brand-label">
			Consultado el {fmtDate(data.hub.data.generado_en, true)} · "Datos al día" significa que son posteriores
			a la última corrida programada.
		</p>
	{/if}
	<div class="flex flex-wrap gap-3">
		{#each MODULES as nav (nav.key)}
			<ModuleCard {nav} modulo={byKey[nav.key]} />
		{/each}
	</div>
	{#if atrasados.length > 0}
		<div class="mt-3">
			<Notice kind="warning" title="Hay módulos sin actualizar">
				<p>
					{atrasados.map((m) => m.titulo).join(', ')}: en desarrollo se actualizan a pedido con
					<code>correr_pipelines.bat</code> (o <code>fogradar job --todos</code>).
				</p>
			</Notice>
		</div>
	{/if}
</Section>
