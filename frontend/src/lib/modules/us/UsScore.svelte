<!-- Score de salud 0-100: barra + número con el color de su señal (≥ 60 / 45-60 / < 45). -->
<script lang="ts">
	import { cn } from '$lib/utils';
	import { fmtScore, scoreClass, scoreSignal } from './format';

	let { score }: { score: number | null | undefined } = $props();

	const valid = $derived(typeof score === 'number' && Number.isFinite(score));
	const pct = $derived(valid ? Math.max(0, Math.min(100, score as number)) : 0);
	const FILL = {
		positive: 'bg-brand-green',
		mixed: 'bg-brand-accent',
		negative: 'bg-brand-red',
		neutral: 'bg-brand-neutral'
	} as const;
</script>

{#if valid}
	<div class="flex items-center justify-end gap-2">
		<div class="h-2.5 w-16 overflow-hidden rounded bg-brand-border xl:w-24" aria-hidden="true">
			<div class={cn('h-full', FILL[scoreSignal(score)])} style:width="{pct}%"></div>
		</div>
		<span class={cn('w-10 text-right tabular-nums', scoreClass(score))}>{fmtScore(score)}</span>
	</div>
{:else}
	<span class="text-brand-muted">—</span>
{/if}
