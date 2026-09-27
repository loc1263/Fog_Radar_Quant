<script lang="ts">
	import type { ModuleStatus } from '$lib/api/types';
	import {
		DataTable,
		ErrorState,
		KpiCard,
		KpiGrid,
		Section,
		SignalBadge
	} from '$lib/components/shared';
	import { fmtDate, fmtDuration } from '$lib/format';
	import type { Column } from '$lib/table';

	let { data } = $props();

	type Row = {
		title: string;
		source: string;
		phase: number;
		migrated: boolean;
		data: boolean;
		job: string | null;
		jobAt: string | null;
		duration: number | null;
	};

	function toRow(m: ModuleStatus): Row {
		return {
			title: m.title,
			source: m.source_project,
			phase: m.migration_phase,
			migrated: m.migrated,
			data: m.data_available,
			job: m.latest_job?.status ?? null,
			jobAt: m.latest_job?.started_at ?? null,
			duration: m.latest_job?.duration_s ?? null
		};
	}

	const rows = $derived(data.status.ok ? data.status.data.modules.map(toRow) : []);
	const migrated = $derived(rows.filter((r) => r.migrated).length);

	const columns: Column<Row>[] = [
		{ key: 'title', label: 'Módulo', wrap: true },
		{ key: 'source', label: 'Proyecto original', wrap: true },
		{ key: 'phase', label: 'Fase', numeric: true },
		{ key: 'migrated', label: 'Migración' },
		{ key: 'data', label: 'Datos' },
		{ key: 'job', label: 'Último job' },
		{ key: 'jobAt', label: 'Inicio', format: (v) => fmtDate(v as string | null, true) },
		{
			key: 'duration',
			label: 'Duración',
			numeric: true,
			format: (v) => fmtDuration(v as number | null)
		}
	];

	const JOB = {
		success: { signal: 'positive', label: 'OK' },
		error: { signal: 'negative', label: 'Error' },
		running: { signal: 'mixed', label: 'En curso' },
		stale: { signal: 'negative', label: 'Colgado' }
	} as const;
</script>

<Section title="Backend">
	{#if data.health.ok}
		<KpiGrid>
			<KpiCard label="Estado" value="En línea" tone="green" />
			<KpiCard label="Versión API" value={data.health.data.version} />
			<KpiCard label="Entorno" value={data.health.data.env} tone="accent" />
			<KpiCard label="Módulos migrados" value="{migrated} / {rows.length || 4}" tone="blue" />
		</KpiGrid>
	{:else}
		<ErrorState
			title="Backend no disponible"
			message={data.health.message}
			code={data.health.code}
		/>
	{/if}
</Section>

{#if data.status.ok}
	<Section title="Módulos y últimas corridas">
		<DataTable {rows} {columns} csvFilename="estado-modulos">
			{#snippet cell({ column, row, text })}
				{#if column.key === 'migrated'}
					<SignalBadge
						signal={row.migrated ? 'positive' : 'mixed'}
						label={row.migrated ? 'Completa' : 'Pendiente'}
					/>
				{:else if column.key === 'data'}
					<SignalBadge
						signal={row.data ? 'positive' : 'neutral'}
						label={row.data ? 'Sí' : 'Sin datos'}
					/>
				{:else if column.key === 'job'}
					{#if row.job && row.job in JOB}
						{@const j = JOB[row.job as keyof typeof JOB]}
						<SignalBadge signal={j.signal} label={j.label} />
					{:else}
						<span class="text-brand-muted">—</span>
					{/if}
				{:else}
					{text}
				{/if}
			{/snippet}
		</DataTable>
	</Section>
{/if}
