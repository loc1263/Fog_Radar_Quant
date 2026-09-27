<!-- Ticker que abre su página de Yahoo Finance en otra pestaña. -->
<script lang="ts">
	let { ticker }: { ticker: string } = $props();

	const symbol = $derived(encodeURIComponent(ticker));
</script>

<a
	href="https://finance.yahoo.com/quote/{symbol}"
	target="_blank"
	rel="noopener noreferrer"
	class="font-bold whitespace-nowrap text-brand-label hover:text-brand-accent hover:underline"
	title="Ver {ticker} en Yahoo Finance"
>
	{ticker}
</a>
