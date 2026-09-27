<!-- Buscador de ticker para la evaluación en vivo (GET: funciona sin JavaScript). -->
<script lang="ts">
	import { resolve } from '$app/paths';

	let { value = '' }: { value?: string } = $props();
</script>

<form method="GET" action={resolve('/eeuu/ticker')} class="flex items-end gap-2" role="search">
	<label class="block">
		<span class="mb-1 block text-sm font-bold text-brand-label">Evaluar un ticker en vivo</span>
		<input
			name="t"
			{value}
			required
			maxlength="15"
			pattern="[A-Za-z0-9.\-^=]+"
			placeholder="AAPL, BRK-B…"
			autocomplete="off"
			class="w-40 rounded border border-brand-border bg-brand-bg px-2 py-1.5 text-sm uppercase"
		/>
	</label>
	<button
		type="submit"
		class="rounded border border-brand-strong bg-brand-strong px-3 py-1.5 text-sm font-bold text-white hover:opacity-90"
		>Evaluar</button
	>
</form>
