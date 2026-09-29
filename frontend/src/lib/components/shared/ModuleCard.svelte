<!-- Tarjeta de un módulo en el hub: estado de datos y de la última corrida. -->
<script lang="ts">
	import { resolve } from '$app/paths';
	import { ruta } from '$lib/nav-href';
	import type { ModuleStatus } from '$lib/api/types';
	import type { ModuleNav } from '$lib/nav';
	import { fmtDate } from '$lib/format';
	import SignalBadge from './SignalBadge.svelte';

	let { nav, status }: { nav: ModuleNav; status?: ModuleStatus } = $props();

	const badge = $derived.by(() => {
		if (!status) return { signal: 'neutral' as const, label: 'Sin estado' };
		if (!status.migrated)
			return { signal: 'mixed' as const, label: `Migración · Fase ${status.migration_phase}` };
		if (status.latest_job?.status === 'error')
			return { signal: 'negative' as const, label: 'Última corrida con error' };
		if (status.data_available) return { signal: 'positive' as const, label: 'Datos al día' };
		return { signal: 'neutral' as const, label: 'Sin datos' };
	});
</script>

<a
	href={resolve(ruta(nav.href))}
	class="flex min-w-[240px] flex-1 flex-col gap-2 rounded border border-brand-border bg-brand-card p-4 hover:border-brand-label"
>
	<div class="flex items-start justify-between gap-2">
		<span class="text-lg font-bold">{nav.label}</span>
		<SignalBadge signal={badge.signal} label={badge.label} />
	</div>
	<p class="text-sm text-brand-muted">{nav.description}</p>
	<dl class="mt-auto grid grid-cols-2 gap-x-3 gap-y-1 text-[12.5px]">
		<dt class="text-brand-label">Origen</dt>
		<dd class="text-right">{status?.source_project ?? '—'}</dd>
		<dt class="text-brand-label">Última corrida OK</dt>
		<dd class="text-right tabular-nums">{fmtDate(status?.last_success?.finished_at, true)}</dd>
	</dl>
</a>
