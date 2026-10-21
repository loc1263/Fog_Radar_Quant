<!-- Ticker con enlace a StockAnalysis + acceso a la evaluación en vivo en el sitio. -->
<script lang="ts">
	import { resolve } from '$app/paths';
	import { TickerLink } from '$lib/components/shared';

	let { ticker }: { ticker: string } = $props();
</script>

<span class="inline-flex items-center gap-1.5 whitespace-nowrap">
	<TickerLink {ticker} />
	<a
		href={resolve('/eeuu/ticker/[ticker]', { ticker })}
		class="text-xs text-brand-muted hover:text-brand-accent hover:underline"
		title="Evaluar {ticker} en vivo">evaluar</a
	>
</span>
