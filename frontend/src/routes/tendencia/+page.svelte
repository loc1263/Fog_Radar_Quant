<script lang="ts">
	import { ModuleStatusPanel } from '$lib/components/shared';

	let { data } = $props();
</script>

<ModuleStatusPanel status={data.status} title="Tendencia Asia + EEUU" />
