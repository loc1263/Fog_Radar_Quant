<!-- Señal técnica (Alcista / Bajista / Lateral / mixta). Sin señal: guion. -->
<script lang="ts">
	import { SignalBadge } from '$lib/components/shared';
	import { senal } from './format';

	let { valor }: { valor?: string | null } = $props();
</script>

{#if valor}
	<SignalBadge signal={senal(valor)} label={valor} />
{:else}
	<span class="text-brand-muted">—</span>
{/if}
