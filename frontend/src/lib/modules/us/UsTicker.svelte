<!-- Ticker de EEUU con enlace a StockAnalysis. -->
<script lang="ts">
	import { TickerLink } from '$lib/components/shared';

	let { ticker }: { ticker: string } = $props();
</script>

<TickerLink {ticker} />
