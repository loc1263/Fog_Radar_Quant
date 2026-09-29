<script lang="ts">
	import type { TdExcluido } from '$lib/api/types';
	import { DataTable, Notice, Section } from '$lib/components/shared';
	import type { Column } from '$lib/table';
	import { fmtNumber } from '$lib/format';

	let { data } = $props();

	const d = $derived(data.latest.ok ? data.latest.data : null);
	const principal = $derived(d?.analyses[0] ?? null);
	const excluidos = $derived(d?.excluidos ?? []);
	const columns: Column<TdExcluido>[] = [
		{ key: 'ticker', label: 'Ticker' },
		{ key: 'nombre', label: 'Empresa', wrap: true },
		{ key: 'motivo', label: 'Motivo', wrap: true }
	];
</script>

<Section title="Limitaciones y advertencias">
	<Notice kind="warning">
		<p>
			Este dashboard es un producto cuantitativo (MVP) y no constituye asesoría financiera ni
			recomendación de inversión. Consulta a un asesor financiero antes de tomar decisiones.
		</p>
		<p>
			El modelo de Markowitz usa retornos y covarianzas históricos de 5 años como estimador del
			comportamiento futuro; esto no garantiza que el mismo patrón se repita, y tiende a
			sobreponderar activos que tuvieron un desempeño excepcional recientemente.
		</p>
		{#if principal}
			<p>
				El Sharpe Ratio del portafolio principal ({fmtNumber(
					principal.performance.portafolio_15_finales.sharpe_ratio,
					2
				)}) refleja el período de mercado usado para estimarlo; no debe interpretarse como un
				retorno esperado garantizado hacia adelante.
				{#if (d?.analyses.length ?? 0) > 1}
					Lo mismo aplica a cualquier análisis complementario incluido en este dashboard.
				{/if}
			</p>
		{/if}
		<p>
			Los precios de Asia se convierten a USD con el tipo de cambio de Yahoo Finance; los tickers
			sin historia suficiente o con moneda no soportada quedan fuera del análisis.
		</p>
	</Notice>
</Section>

{#if excluidos.length > 0}
	<Section title="Tickers excluidos del análisis">
		<p>
			Estas acciones del universo descubierto quedaron fuera en el último armado del dataset (sin
			historia suficiente para estimar la covarianza, sin datos o sin tipo de cambio para llevarlas
			a USD).
		</p>
		<DataTable
			rows={excluidos}
			{columns}
			csvFilename="tendencia_excluidos_{d?.run_dir}"
			caption="Tickers excluidos"
		/>
	</Section>
{/if}
