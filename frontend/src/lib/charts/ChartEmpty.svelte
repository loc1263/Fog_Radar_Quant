<script lang="ts">
	let { height = 240 }: { height?: number } = $props();
</script>

<div
	class="flex items-center justify-center rounded bg-brand-card text-sm text-brand-muted"
	style:height="{height}px"
>
	Sin datos para este rango.
</div>
