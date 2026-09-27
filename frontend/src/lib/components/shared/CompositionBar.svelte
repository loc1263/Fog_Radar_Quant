<!-- Barra 100 % apilada + leyenda (§5.8). Usa la paleta de marca para composición. -->
<script lang="ts">
	import { fmtPct } from '$lib/format';

	interface Segment {
		label: string;
		/** Fracción 0-1. */
		value: number;
	}

	let { segments }: { segments: Segment[] } = $props();

	const PALETTE = ['#1B5EA5', '#9A6E00', '#1A7A55', '#B83232', '#1B3A6B'];

	const sorted = $derived(
		[...segments].filter((s) => s.value > 0).sort((a, b) => b.value - a.value)
	);
	const total = $derived(sorted.reduce((acc, s) => acc + s.value, 0));
</script>

<div>
	<div
		class="flex h-[22px] overflow-hidden rounded border border-brand-border"
		role="img"
		aria-label="Composición"
	>
		{#each sorted as seg, i (seg.label)}
			<div
				style:width="{total ? (seg.value / total) * 100 : 0}%"
				style:background-color={PALETTE[i % PALETTE.length]}
				title="{seg.label} ({fmtPct(seg.value)})"
			></div>
		{/each}
	</div>
	<div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm text-brand-label">
		{#each sorted as seg, i (seg.label)}
			<span class="inline-flex items-center gap-1.5">
				<span class="h-2.5 w-2.5 rounded-sm" style:background-color={PALETTE[i % PALETTE.length]}
				></span>
				{seg.label} ({fmtPct(seg.value)})
			</span>
		{/each}
	</div>
</div>
