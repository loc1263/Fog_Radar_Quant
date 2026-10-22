<!--
  Composición de un portafolio (§5.6): barras relativas al peso máximo, orden
  descendente y numeradas. Con `tickers` se muestran todos (incluso 0 %); si no, se ocultan
  los pesos despreciables.
-->
<script lang="ts">
	import { WeightBar } from '$lib/components/shared';
	import { fmtShare } from './format';

	let { weights, tickers }: { weights: Record<string, number>; tickers?: string[] } = $props();

	const rows = $derived(
		(tickers
			? tickers.map((t) => [t, weights[t] ?? 0] as const)
			: Object.entries(weights).filter(([, w]) => w > 0.0005)
		).toSorted(([, a], [, b]) => b - a)
	);
	const max = $derived(rows.length ? rows[0][1] : 1);
</script>

{#if rows.length === 0}
	<p class="text-sm text-brand-muted">El portafolio no asignó peso a ningún ticker.</p>
{:else}
	<ul class="space-y-1.5" aria-label="Composición del portafolio">
		{#each rows as [ticker, weight], i (ticker)}
			<li class="flex items-center gap-2 text-sm">
				<span class="w-6 shrink-0 text-right text-brand-muted tabular-nums">{i + 1}</span>
				<span class="w-16 shrink-0 font-num">{ticker}</span>
				<div class="flex-1"><WeightBar value={weight} {max} label={fmtShare(weight)} /></div>
			</li>
		{/each}
	</ul>
{/if}
