<!--
  Selector de análisis (Asia + EEUU / solo EEUU) como enlaces: el análisis vive en
  la URL (`?analisis=us`), así cada vista es compartible y funciona sin JavaScript.
-->
<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { ruta } from '$lib/nav-href';
	import type { TdAnalisis } from '$lib/api/types';
	import { cn } from '$lib/utils';
	import { analisisSlug } from './format';

	let { analyses, actual }: { analyses: TdAnalisis[]; actual: TdAnalisis | null } = $props();

	function href(a: TdAnalisis): string {
		const params = new URL(page.url).searchParams;
		const slug = analisisSlug(a.key);
		if (slug) params.set('analisis', slug);
		else params.delete('analisis');
		const q = params.toString();
		return q ? `?${q}` : '';
	}
	// Las vistas de la corrida no tienen parámetros de ruta: su id sirve de base del href.
</script>

{#if analyses.length > 1}
	<div class="flex flex-wrap items-center gap-2" role="group" aria-label="Análisis">
		<span class="text-xs font-bold tracking-wide text-brand-accent uppercase">Análisis</span>
		{#each analyses as a (a.key)}
			{@const activo = a.key === actual?.key}
			<a
				href="{resolve(ruta(page.route.id ?? '/tendencia'))}{href(a)}"
				data-sveltekit-noscroll
				aria-current={activo ? 'true' : undefined}
				class={cn(
					'rounded border px-2.5 py-0.5 text-xs font-bold',
					activo
						? 'border-brand-strong bg-brand-strong text-white'
						: 'border-brand-border bg-brand-bg text-brand-label hover:bg-brand-card'
				)}>{a.label}</a
			>
		{/each}
	</div>
{/if}
