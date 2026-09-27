<!-- Score 0-100 con barra: verde ≥ 70, ámbar 40-69, rojo < 40. -->
<script lang="ts">
	import { cn } from '$lib/utils';

	let { score }: { score: number | null | undefined } = $props();

	const valid = $derived(typeof score === 'number' && Number.isFinite(score));
	const pct = $derived(valid ? Math.max(0, Math.min(100, score as number)) : 0);
	const fill = $derived(
		pct >= 70 ? 'bg-brand-green' : pct >= 40 ? 'bg-brand-accent' : 'bg-brand-red'
	);
</script>

{#if valid}
	<div class="flex items-center justify-end gap-2">
		<div class="h-2.5 w-20 overflow-hidden rounded bg-brand-border" aria-hidden="true">
			<div class={cn('h-full', fill)} style:width="{pct}%"></div>
		</div>
		<span class="w-8 text-right font-bold tabular-nums">{Math.round(pct)}</span>
	</div>
{:else}
	<span class="text-brand-muted">—</span>
{/if}
