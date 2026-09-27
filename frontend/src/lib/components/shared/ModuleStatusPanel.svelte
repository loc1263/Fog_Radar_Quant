<!-- Estado de un módulo aún no migrado (placeholder de las Fases 3-6). -->
<script lang="ts">
	import type { Loaded, ModuleStatus } from '$lib/api/types';
	import { fmtDate, fmtDuration } from '$lib/format';
	import ErrorState from './ErrorState.svelte';
	import KpiCard from './KpiCard.svelte';
	import KpiGrid from './KpiGrid.svelte';
	import Notice from './Notice.svelte';
	import Section from './Section.svelte';

	let { status, title }: { status: Loaded<ModuleStatus>; title: string } = $props();
</script>

{#if status.ok}
	{@const s = status.data}
	<Section title="{title} — estado">
		<KpiGrid>
			<KpiCard
				label="Migración"
				value={s.migrated ? 'Completa' : `Fase ${s.migration_phase}`}
				tone={s.migrated ? 'green' : 'accent'}
			/>
			<KpiCard
				label="Datos disponibles"
				value={s.data_available ? 'Sí' : 'No'}
				tone={s.data_available ? 'green' : 'text'}
			/>
			<KpiCard label="Última corrida OK" value={fmtDate(s.last_success?.finished_at, true)} />
			<KpiCard label="Duración" value={fmtDuration(s.last_success?.duration_s)} tone="blue" />
		</KpiGrid>
		{#if !s.migrated}
			<Notice kind="warning" title="Módulo en migración">
				<p>
					Este módulo viene del proyecto <strong>{s.source_project}</strong> y se migra en la Fase {s.migration_phase}.
					Mientras tanto, el proyecto original sigue siendo la referencia.
				</p>
			</Notice>
		{/if}
	</Section>
{:else}
	<ErrorState message={status.message} code={status.code} />
{/if}
