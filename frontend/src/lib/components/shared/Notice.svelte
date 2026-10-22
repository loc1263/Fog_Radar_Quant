<!-- Cajas de mensaje (§5.9). `size="compact"` para notas informativas de una o dos líneas. -->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cn } from '$lib/utils';

	type Kind = 'info' | 'warning' | 'neutral';

	let {
		kind = 'info',
		title,
		size = 'normal',
		children
	}: { kind?: Kind; title?: string; size?: 'normal' | 'compact'; children: Snippet } = $props();

	const BG: Record<Kind, string> = {
		info: 'bg-brand-blue-bg',
		warning: 'bg-brand-amber-bg',
		neutral: 'bg-brand-card'
	};
</script>

<div
	class={cn(
		'rounded border border-brand-border',
		size === 'compact' ? 'space-y-1 px-3 py-2 text-xs' : 'space-y-2 p-4 text-sm',
		BG[kind]
	)}
	role="note"
>
	{#if title}<div class="font-bold text-brand-label">{title}</div>{/if}
	{@render children()}
</div>
