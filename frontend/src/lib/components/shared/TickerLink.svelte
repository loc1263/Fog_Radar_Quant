<!-- Ticker que abre su página de StockAnalysis en otra pestaña. `etf` usa la sección de ETFs para tickers de EEUU. -->
<script lang="ts">
	import { stockAnalysisPath } from '$lib/stockanalysis';

	let { ticker, etf = false }: { ticker: string; etf?: boolean } = $props();

	const path = $derived(stockAnalysisPath(ticker, etf));
</script>

<a
	href="https://stockanalysis.com{path}"
	target="_blank"
	rel="noopener noreferrer"
	class="font-num whitespace-nowrap text-brand-label hover:text-brand-accent hover:underline"
	title="Ver {ticker} en StockAnalysis"
>
	{ticker}
</a>
