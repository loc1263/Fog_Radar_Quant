<!-- Badge de estado (§5.7). Verde/rojo/ámbar solo con ese significado. -->
<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';

	type Signal = 'positive' | 'negative' | 'mixed' | 'neutral' | 'category';

	let { signal, label }: { signal: Signal; label: string } = $props();
</script>

<Badge variant={signal}>{label}</Badge>
