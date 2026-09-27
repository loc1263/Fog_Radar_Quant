<!-- Barra de peso en una celda (§5.6). El ancho es relativo al máximo de la columna. -->
<script lang="ts">
	let { value, max, label }: { value: number; max: number; label: string } = $props();

	const width = $derived(max > 0 ? Math.max(0, Math.min(100, (value / max) * 100)) : 0);
</script>

<div class="flex items-center justify-end gap-2">
	<div class="h-2.5 w-24 overflow-hidden rounded bg-brand-border" aria-hidden="true">
		<div class="h-full bg-brand-blue" style:width="{width}%"></div>
	</div>
	<span class="w-14 text-right tabular-nums">{label}</span>
</div>
