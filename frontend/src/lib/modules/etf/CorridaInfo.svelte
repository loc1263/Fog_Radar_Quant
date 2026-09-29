<!-- Línea con la fecha de la corrida que se está mostrando. -->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import { fmtFechaCorrida } from './format';

	let { fecha, children }: { fecha?: string | null; children?: Snippet } = $props();
</script>

<p class="text-sm text-brand-muted">
	Corrida del <span class="font-bold text-brand-text">{fmtFechaCorrida(fecha)}</span>
	{#if children}· {@render children()}{/if}
</p>
