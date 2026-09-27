<!--
  Tarjeta KPI (§5.4). `tone` según lo que representa: green = rendimiento,
  blue = riesgo/volatilidad, accent = métrica estrella, text = contexto,
  red = alerta. Con `signedValue` el color sale del signo.
-->
<script lang="ts">
	import { cn } from '$lib/utils';

	type Tone = 'green' | 'red' | 'blue' | 'accent' | 'text';

	interface Props {
		label: string;
		value: string;
		tone?: Tone;
		signedValue?: number | null;
		hint?: string;
	}

	let { label, value, tone = 'text', signedValue, hint }: Props = $props();

	const TONES: Record<Tone, string> = {
		green: 'text-brand-green',
		red: 'text-brand-red',
		blue: 'text-brand-blue',
		accent: 'text-brand-accent',
		text: 'text-brand-text'
	};

	const effective = $derived<Tone>(
		typeof signedValue === 'number' && Number.isFinite(signedValue)
			? signedValue >= 0
				? 'green'
				: 'red'
			: tone
	);
</script>

<div class="min-w-[160px] flex-1 rounded border border-brand-border bg-brand-card p-4">
	<div class="text-[12.5px] text-brand-label">{label}</div>
	<div class={cn('mt-1.5 text-xl font-bold tabular-nums', TONES[effective])}>{value}</div>
	{#if hint}<div class="mt-1 text-xs text-brand-muted">{hint}</div>{/if}
</div>
