<!--
  z-score de un pilar: pill sólida si es fuerte (|z| > 1,2), texto de color si
  es moderado (|z| > 0,15) y neutro cerca de cero. Verde/rojo solo por signo.
-->
<script lang="ts">
	import { cn } from '$lib/utils';
	import { fmtZ } from './format';

	let { value }: { value: number | null | undefined } = $props();

	const valid = $derived(typeof value === 'number' && Number.isFinite(value));
	const tone = $derived.by(() => {
		if (!valid) return 'text-brand-muted';
		const v = value as number;
		if (v > 1.2) return 'bg-brand-green-solid text-white';
		if (v < -1.2) return 'bg-brand-red-solid text-white';
		if (v > 0.15) return 'text-brand-green';
		if (v < -0.15) return 'text-brand-red';
		return 'text-brand-muted';
	});
</script>

<span
	class={cn(
		'inline-flex min-w-14 justify-center rounded-full px-2 py-0.5 text-xs font-bold tabular-nums',
		tone
	)}
	title={valid ? undefined : 'Sin métricas disponibles en este pilar'}
>
	{valid ? fmtZ(value) : '—'}
</span>
