<!-- Posición del precio dentro del rango de 52 semanas (port de Rango52SemanasBar.tsx). -->
<script lang="ts">
	import { cn } from '$lib/utils';
	import { rango52 } from './format';

	interface Props {
		precio: number | null | undefined;
		max: number | null | undefined;
		min: number | null | undefined;
	}

	let { precio, max, min }: Props = $props();

	const r = $derived(
		rango52({ precio: precio ?? null, max_52s: max ?? null, min_52s: min ?? null })
	);
	const TONO = { accent: 'text-brand-accent', blue: 'text-brand-blue', label: 'text-brand-label' };
</script>

{#if r}
	<div class="flex min-w-[88px] flex-col gap-1" title={r.title}>
		<div class="relative h-2.5 w-full rounded bg-brand-border" aria-hidden="true">
			<div class="absolute inset-y-0 left-0 rounded bg-brand-blue" style:width="{r.pos}%"></div>
			<div
				class="absolute top-1/2 h-3.5 w-1 -translate-x-1/2 -translate-y-1/2 rounded-sm bg-brand-text"
				style:left="{r.pos}%"
			></div>
		</div>
		<span class={cn('text-xs leading-tight font-num', TONO[r.tono])}>{r.etiqueta}</span>
		<span class="sr-only">{r.title}</span>
	</div>
{:else}
	<span class="text-xs text-brand-muted">N/D</span>
{/if}
