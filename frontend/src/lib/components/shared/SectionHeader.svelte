<!-- Título de sección (§5.3): mayúsculas, ámbar, línea fina debajo. -->
<script lang="ts">
	import type { Snippet } from 'svelte';

	let { children, id }: { children: Snippet; id?: string } = $props();
</script>

<h2 {id} class="section-header">{@render children()}</h2>
