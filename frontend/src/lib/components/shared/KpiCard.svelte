<!--
  Tarjeta KPI (§5.4). `tone` según lo que representa: green = rendimiento,
  blue = riesgo/volatilidad, accent = métrica estrella, text = contexto,
  red = alerta. Con `signedValue` el color sale del signo. Con `href` (ruta del sitio)
  la tarjeta entera es un enlace a su sección; `search` agrega `?a=1` conservado.
-->
<script lang="ts">
	import type { Pathname } from '$app/types';
	import { resolve } from '$app/paths';
	import { ruta } from '$lib/nav-href';
	import { cn } from '$lib/utils';

	type Tone = 'green' | 'red' | 'blue' | 'accent' | 'text';

	interface Props {
		label: string;
		value: string;
		tone?: Tone;
		signedValue?: number | null;
		hint?: string;
		href?: Pathname;
		search?: string;
	}

	let { label, value, tone = 'text', signedValue, hint, href, search = '' }: Props = $props();

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

<!-- overflow-wrap: un valor largo (p. ej. MALLPLAZA.SN o un nombre de archivo) no desborda la tarjeta. -->
{#snippet cuerpo()}
	<div class="text-[12.5px] text-brand-label">{label}</div>
	<div class={cn('mt-1.5 text-xl font-num tabular-nums', TONES[effective])}>{value}</div>
	{#if hint}<div class="mt-1 text-xs text-brand-muted">{hint}</div>{/if}
{/snippet}

{#if href}
	<a
		href="{resolve(ruta(href))}{search}"
		class="block min-w-[180px] flex-1 rounded border border-brand-border bg-brand-card p-4 [overflow-wrap:anywhere] transition-colors hover:border-brand-accent focus-visible:border-brand-accent focus-visible:outline-none"
	>
		{@render cuerpo()}
	</a>
{:else}
	<div
		class="min-w-[180px] flex-1 rounded border border-brand-border bg-brand-card p-4 [overflow-wrap:anywhere]"
	>
		{@render cuerpo()}
	</div>
{/if}
