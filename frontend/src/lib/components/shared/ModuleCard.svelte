<!-- Tarjeta de un módulo en el hub: frescura de los datos, top 3 y próxima corrida. -->
<script lang="ts">
	import { resolve } from '$app/paths';
	import { ruta } from '$lib/nav-href';
	import type { HubModulo } from '$lib/api/types';
	import type { ModuleNav } from '$lib/nav';
	import { fmtDate } from '$lib/format';
	import { FRESCURA, fmtEdad, fmtValor } from '$lib/transversal';
	import SignalBadge from './SignalBadge.svelte';

	let { nav, modulo }: { nav: ModuleNav; modulo?: HubModulo } = $props();

	const badge = $derived.by(() => {
		if (!modulo) return { signal: 'neutral' as const, label: 'Sin estado' };
		if (modulo.ultimo_job?.status === 'running')
			return { signal: 'mixed' as const, label: 'Actualizando…' };
		if (modulo.ultimo_job?.status === 'error' && modulo.frescura !== 'al_dia')
			return { signal: 'negative' as const, label: 'Última corrida con error' };
		return FRESCURA[modulo.frescura];
	});
</script>

<a
	href={resolve(ruta(nav.href))}
	class="flex min-w-[260px] flex-1 flex-col gap-3 rounded border border-brand-border bg-brand-card p-4 hover:border-brand-label"
>
	<div class="flex items-start justify-between gap-2">
		<span class="text-lg font-bold">{nav.label}</span>
		<SignalBadge signal={badge.signal} label={badge.label} />
	</div>
	<p class="text-sm">{nav.description}</p>

	{#if modulo && modulo.top.length > 0}
		<div>
			<div class="mb-1 text-[12.5px] font-bold text-brand-label">
				Top 3 · {modulo.metrica}
			</div>
			<ol class="space-y-1 text-sm">
				{#each modulo.top as item, i (item.ticker)}
					<li class="flex items-baseline justify-between gap-2">
						<span class="min-w-0 truncate">
							<span class="font-bold">{i + 1}. {item.ticker}</span>
							{#if item.nombre}<span class="text-brand-label"> · {item.nombre}</span>{/if}
						</span>
						<span class="whitespace-nowrap tabular-nums">{fmtValor(item.valor, modulo.unidad)}</span
						>
					</li>
				{/each}
			</ol>
		</div>
	{:else if modulo?.mensaje}
		<p class="text-sm">{modulo.mensaje}</p>
	{/if}

	<dl class="mt-auto grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-[12.5px]">
		<dt class="text-brand-label">Datos al</dt>
		<dd class="text-right tabular-nums">
			<span class="whitespace-nowrap">{fmtDate(modulo?.datos_al, true)}</span>
			{#if modulo?.edad_horas !== null && modulo?.edad_horas !== undefined}
				<span class="block whitespace-nowrap">{fmtEdad(modulo.edad_horas)}</span>
			{/if}
		</dd>
		{#if modulo?.universo}
			<dt class="text-brand-label">Universo</dt>
			<dd class="text-right tabular-nums">{modulo.universo}</dd>
		{/if}
		<dt class="text-brand-label">Se actualiza</dt>
		<dd class="text-right">{modulo?.programa ?? '—'}</dd>
		<dt class="text-brand-label">Próxima</dt>
		<dd class="text-right whitespace-nowrap tabular-nums">{fmtDate(modulo?.proxima, true)}</dd>
	</dl>
</a>
