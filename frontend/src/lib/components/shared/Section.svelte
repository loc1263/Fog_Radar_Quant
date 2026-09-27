<!-- Bloque de contenido con su título (§4: secciones separadas por mt-10). -->
<script lang="ts">
	import type { Snippet } from 'svelte';
	import SectionHeader from './SectionHeader.svelte';

	let { title, children, id }: { title: string; children: Snippet; id?: string } = $props();
</script>

<section class="mt-10 first:mt-6" aria-labelledby={id}>
	<SectionHeader {id}>{title}</SectionHeader>
	<div class="space-y-4">{@render children()}</div>
</section>
