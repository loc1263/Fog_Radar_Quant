<!--
  Sliders de pesos por pilar. El recálculo ocurre en el navegador; el % es la
  participación real de cada pilar en el Score Compuesto con los pesos actuales.
-->
<script lang="ts">
	import type { ClPilarInfo, ClPilarKey } from '$lib/api/types';
	import { Button } from '$lib/components/ui/button';
	import { fmtNumber } from '$lib/format';
	import {
		PESO_MAX,
		PESO_MIN,
		PESO_PASO,
		participacion,
		pesosIguales,
		type Pesos
	} from './scoring';

	interface Props {
		pilares: ClPilarInfo[];
		pesos: Pesos;
		defaults: Pesos;
		onchange: (key: ClPilarKey, value: number) => void;
		onreset: () => void;
	}

	let { pilares, pesos, defaults, onchange, onreset }: Props = $props();

	const share = $derived(participacion(pesos));
	const sonDefault = $derived(pesosIguales(pesos, defaults));
</script>

<div class="rounded border border-brand-border bg-brand-card p-4">
	<div class="grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-3 xl:grid-cols-6">
		{#each pilares as pilar (pilar.key)}
			<div class="space-y-1.5">
				<div class="flex items-baseline justify-between text-sm">
					<label for="peso-{pilar.key}" class="font-bold text-brand-label">{pilar.label}</label>
					<span class="tabular-nums">{fmtNumber(pesos[pilar.key])}</span>
				</div>
				<input
					id="peso-{pilar.key}"
					type="range"
					min={PESO_MIN}
					max={PESO_MAX}
					step={PESO_PASO}
					value={pesos[pilar.key]}
					oninput={(e) => onchange(pilar.key, Number(e.currentTarget.value))}
					class="w-full cursor-pointer accent-brand-accent"
				/>
				<div class="text-right text-xs font-bold text-brand-accent tabular-nums">
					{fmtNumber(share[pilar.key], 1)} % del score
				</div>
			</div>
		{/each}
	</div>
	<div class="mt-4 flex flex-wrap items-center justify-between gap-2 text-sm">
		<span class="text-brand-muted">
			{sonDefault
				? 'Pesos por defecto del modelo.'
				: 'Pesos personalizados: el ranking se recalculó en tu navegador.'}
		</span>
		<Button size="sm" onclick={onreset} disabled={sonDefault}>Restablecer</Button>
	</div>
</div>
