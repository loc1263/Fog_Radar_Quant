<!-- Cajas de mensaje (§5.9). -->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cn } from '$lib/utils';

	type Kind = 'info' | 'warning' | 'neutral';

	let {
		kind = 'info',
		title,
		children
	}: { kind?: Kind; title?: string; children: Snippet } = $props();

	const BG: Record<Kind, string> = {
		info: 'bg-brand-blue-bg',
		warning: 'bg-brand-amber-bg',
		neutral: 'bg-brand-card'
	};
</script>

<div class={cn('space-y-2 rounded border border-brand-border p-4 text-sm', BG[kind])} role="note">
	{#if title}<div class="font-bold text-brand-label">{title}</div>{/if}
	{@render children()}
</div>
