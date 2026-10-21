<script lang="ts">
	import { page } from '$app/state';
	import { EmptyState, Section } from '$lib/components/shared';
	import { MomentumTabla, RetornoTabla } from '$lib/modules/td';
	import { elegirAnalisis, sufijoAnalisis } from '$lib/modules/td/format';

	let { data } = $props();

	const d = $derived(data.latest.ok ? data.latest.data : null);
	const a = $derived(d ? elegirAnalisis(d, page.url.searchParams.get('analisis')) : null);
</script>

{#if a}
	<Section title="Top acciones por momentum mensual{sufijoAnalisis(a)}">
		<p>
			Ranking independiente de la selección de Markowitz: evalúa el universo '{a.label}' ({a.universe_size}
			acciones) por fuerza y persistencia de la tendencia mensual (retorno acumulado a 1, 2, 4, 6, 8,
			10 y 12 meses, ponderado por la proporción de meses positivos del último año). Puede incluir acciones
			que el modelo de covarianza descartó por volatilidad o correlación alta, pero que muestran el mejor
			impulso de corto plazo.
		</p>
		{#if a.momentum.length > 0}
			<MomentumTabla
				rows={a.momentum}
				windows={[1, 2, 4, 6, 8, 10, 12]}
				unit="m"
				csvFilename="tendencia_momentum_mensual{a.key}_{d?.run_dir}"
				caption="Momentum mensual"
			/>
		{:else}
			<EmptyState>Sin datos de momentum mensual disponibles.</EmptyState>
		{/if}
	</Section>

	<Section title="Top acciones por momentum semanal{sufijoAnalisis(a)}">
		<p>
			Misma lógica que el ranking mensual, pero sobre velas semanales (5 días hábiles): retorno
			acumulado a 1, 2, 3 y 4 semanas, ponderado por la proporción de semanas positivas de las
			últimas 4. Capta el impulso de muy corto plazo que el ranking mensual suaviza.
		</p>
		{#if a.momentum_semanal.length > 0}
			<MomentumTabla
				rows={a.momentum_semanal}
				windows={[1, 2, 3, 4]}
				unit="s"
				csvFilename="tendencia_momentum_semanal{a.key}_{d?.run_dir}"
				caption="Momentum semanal"
			/>
		{:else}
			<EmptyState>
				Sin datos de momentum semanal disponibles (la corrida es anterior a este ranking).
			</EmptyState>
		{/if}
	</Section>

	<Section title="Mejor rendimiento de los últimos ~4 meses{sufijoAnalisis(a)}">
		<p>
			Las acciones del universo '{a.label}' con mayor retorno puro en las últimas
			{d?.parametros?.retorno_semanas ?? ''} semanas (unos 4 meses, sobre velas semanales). Es un ranking
			solo por rentabilidad reciente: no usa el score ni la consistencia, así que muestra a las que despegaron
			hace poco aunque su tendencia de 12 meses sea modesta. No es una medida ajustada por riesgo.
		</p>
		{#if a.retorno_reciente.length > 0}
			<RetornoTabla
				rows={a.retorno_reciente}
				semanas={d?.parametros?.retorno_semanas}
				csvFilename="tendencia_retorno_reciente{a.key}_{d?.run_dir}"
				caption="Mejor rendimiento de los últimos ~4 meses"
			/>
		{:else}
			<EmptyState>
				Sin datos de retorno reciente disponibles (la corrida es anterior a este ranking).
			</EmptyState>
		{/if}
	</Section>
{/if}
