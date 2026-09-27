<!-- Botón de tema (§5.10): dice a qué modo cambia. -->
<script lang="ts">
	import { onMount } from 'svelte';
	import Moon from '@lucide/svelte/icons/moon';
	import Sun from '@lucide/svelte/icons/sun';
	import { Button } from '$lib/components/ui/button';
	import { theme } from '$lib/theme.svelte';

	onMount(() => theme.init());
</script>

<Button onclick={() => theme.toggle()} aria-label="Cambiar tema">
	{#if theme.current === 'dark'}
		<Sun /> Claro
	{:else}
		<Moon /> Oscuro
	{/if}
</Button>
