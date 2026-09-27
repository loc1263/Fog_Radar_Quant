<script lang="ts">
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
</script>

<div class="rounded border border-brand-border bg-brand-card p-6 text-center text-brand-muted">
	{@render children()}
</div>
