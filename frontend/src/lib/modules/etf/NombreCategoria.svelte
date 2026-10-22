<!-- Nombre del fondo con su categoría Morningstar debajo (celda "Nombre / Categoría"). -->
<script lang="ts">
	let { nombre, categoria }: { nombre?: string | null; categoria?: string | null } = $props();
</script>

<div class="min-w-[12rem]">
	<div class="font-num">{nombre ?? '—'}</div>
	{#if categoria}<div class="text-xs text-brand-label">{categoria}</div>{/if}
</div>
