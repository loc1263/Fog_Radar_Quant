<!--
  Número de ETFs de una categoría; al hacer clic despliega la lista de tickers
  (ordenados por Sharpe) con un botón para copiarlos (port de TickerPopover).
-->
<script lang="ts">
	let { tickers, categoria }: { tickers: string[]; categoria: string } = $props();

	let abierto = $state(false);
	let copiado = $state(false);

	async function copiar() {
		try {
			await navigator.clipboard.writeText(tickers.join(', '));
			copiado = true;
			setTimeout(() => (copiado = false), 1500);
		} catch {
			// Sin permiso de portapapeles: el usuario puede seleccionar el texto.
		}
	}
</script>

<div class="relative inline-block text-left">
	<button
		type="button"
		class="cursor-pointer font-bold tabular-nums underline decoration-brand-label decoration-dotted underline-offset-4"
		aria-expanded={abierto}
		aria-label="Ver los {tickers.length} ETFs de {categoria}"
		onclick={() => (abierto = !abierto)}>{tickers.length}</button
	>
	{#if abierto}
		<div
			class="absolute right-0 z-20 mt-1 flex w-64 flex-col rounded border border-brand-border bg-brand-bg p-3 text-xs shadow-sm"
		>
			<div class="mb-1.5 flex items-center justify-between gap-2">
				<span class="font-bold text-brand-label">{tickers.length} ETFs en la categoría</span>
				<button
					type="button"
					onclick={copiar}
					class="rounded border border-brand-border px-2 py-0.5 font-bold text-brand-label hover:bg-brand-card"
					>{copiado ? 'Copiado' : 'Copiar'}</button
				>
			</div>
			<div class="flex max-h-44 flex-wrap gap-1 overflow-y-auto">
				{#each tickers as t (t)}
					<span class="rounded bg-brand-card px-1.5 py-0.5 text-brand-text">{t}</span>
				{/each}
			</div>
		</div>
	{/if}
</div>
