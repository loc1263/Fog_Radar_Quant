<!-- Error de carga (§5.9): bloque centrado con detalle y "Reintentar". -->
<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';

	interface Props {
		title?: string;
		message: string;
		code?: string;
		retry?: boolean;
	}

	let { title = 'No se pudieron cargar los datos', message, code, retry = true }: Props = $props();
</script>

<div class="mx-auto max-w-2xl p-10 text-center">
	<h2 class="text-xl font-bold text-brand-red">{title}</h2>
	<p class="mt-2 text-brand-muted">{message}</p>
	{#if code}<pre
			class="mt-3 inline-block rounded bg-brand-card p-3 text-left text-sm">{code}</pre>{/if}
	{#if retry}
		<div class="mt-4"><Button onclick={() => invalidateAll()}>Reintentar</Button></div>
	{/if}
</div>
