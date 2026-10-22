<script lang="ts">
	import { page } from '$app/state';
	import { ErrorState } from '$lib/components/shared';
	import { fmtDate } from '$lib/format';
	import { AnalisisSelector } from '$lib/modules/td';
	import { elegirAnalisis } from '$lib/modules/td/format';

	let { data, children } = $props();

	const d = $derived(data.latest.ok ? data.latest.data : null);
	const analisis = $derived(d ? elegirAnalisis(d, page.url.searchParams.get('analisis')) : null);
	const conSelector = $derived(!page.url.pathname.endsWith('/limitaciones'));
</script>

{#if !d}
	{#if !data.latest.ok}
		<div class="mt-10">
			<ErrorState
				title={data.latest.code === 'no_data' ? 'Aún no hay corridas de Tendencia' : undefined}
				message={data.latest.message}
				code={data.latest.code}
			/>
		</div>
	{/if}
{:else}
	<div class="mt-6 flex flex-wrap items-center justify-between gap-3">
		<p class="text-xs text-brand-muted">
			Generado el <span class="font-num text-brand-text">{fmtDate(d.generated_at, true)}</span> ·
			corrida {d.run_dir}
		</p>
		{#if conSelector}<AnalisisSelector analyses={d.analyses} actual={analisis} />{/if}
	</div>
	{@render children()}
{/if}
