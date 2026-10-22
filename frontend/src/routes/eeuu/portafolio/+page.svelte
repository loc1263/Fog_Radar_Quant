<script lang="ts">
	import { resolve } from '$app/paths';
	import { ErrorState, KpiCard, KpiGrid, Section, SectionHeader } from '$lib/components/shared';
	import { fmtNumber, signedClass as signedClassFor } from '$lib/format';
	import { WeightList } from '$lib/modules/us';
	import { fmtRet, fmtScore, fmtShare, scoreClass } from '$lib/modules/us/format';
	import { cn } from '$lib/utils';

	let { data } = $props();

	const precalc = $derived(data.precalc.ok ? data.precalc.data : null);
	const actual = $derived(precalc?.portafolios.find((p) => p.clave === data.clave) ?? null);
	const opt = $derived(actual?.resultado ?? null);

	const pickRows = $derived(
		opt
			? opt.selection
					.map((p) => ({
						...p,
						prior: opt.prior_returns[p.ticker],
						view: opt.views[p.ticker],
						post: opt.posterior_returns[p.ticker],
						w: opt.weights[p.ticker] ?? 0
					}))
					.toSorted((a, b) => b.w - a.w)
			: []
	);
</script>

<Section title="Portafolio">
	<p class="text-xs text-brand-muted">
		<b>Modelo en 3 etapas:</b> (1) ranking por score multi-factor → (2) un representante por cluster de
		correlación → (3) pesos Black-Litterman con tope por activo, long-only.
	</p>
	{#if !data.precalc.ok}
		<ErrorState message={data.precalc.message} code={data.precalc.code} />
	{:else if precalc}
		<nav aria-label="Universo del portafolio" class="mt-2 flex flex-wrap gap-2">
			{#each precalc.portafolios as p (p.clave)}
				<a
					href="{resolve('/eeuu/portafolio')}?u={p.clave}"
					aria-current={p.clave === data.clave ? 'page' : undefined}
					class={cn(
						'rounded border px-3 py-1 text-sm font-bold',
						p.clave === data.clave
							? 'border-brand-strong bg-brand-strong text-white'
							: 'border-brand-border bg-brand-bg text-brand-label hover:bg-brand-card'
					)}>{p.titulo}</a
				>
			{/each}
		</nav>

		{#if opt}
			<div class="mt-4">
				<KpiGrid>
					<KpiCard
						label="Retorno esperado anual"
						value={fmtRet(opt.metrics.expected_return)}
						signedValue={opt.metrics.expected_return}
					/>
					<KpiCard label="Volatilidad" value={fmtShare(opt.metrics.volatility, 2)} tone="blue" />
					<KpiCard label="Sharpe" value={fmtNumber(opt.metrics.sharpe)} tone="accent" />
				</KpiGrid>
			</div>

			<div class="mt-4 grid gap-6 lg:grid-cols-[minmax(0,320px)_minmax(0,1fr)]">
				<div>
					<SectionHeader>Composición</SectionHeader>
					<WeightList weights={opt.weights} />
				</div>
				<section>
					<SectionHeader>Selección por cluster y retornos Black-Litterman</SectionHeader>
					<p class="mb-2 max-w-3xl text-sm text-brand-muted">
						{opt.selection.length} activos de un universo de {opt.universe.length}: el de mayor
						score de cada cluster de correlación. Peso máximo por activo: {fmtShare(
							opt.max_weight,
							0
						)}.
					</p>
					<div class="overflow-x-auto rounded border border-brand-border">
						<table class="w-full border-collapse text-sm">
							<caption class="sr-only"
								>Selección por cluster y retornos de Black-Litterman por ticker</caption
							>
							<thead>
								<tr class="bg-brand-strong text-white">
									<th scope="col" class="px-3 py-1 text-right">Cluster</th>
									<th scope="col" class="px-3 py-1 text-left">Ticker</th>
									<th scope="col" class="px-3 py-1 text-right">Score</th>
									<th scope="col" class="px-3 py-1 text-right">Equilibrio (π)</th>
									<th scope="col" class="px-3 py-1 text-right">View (Q)</th>
									<th scope="col" class="px-3 py-1 text-right">Posterior</th>
									<th scope="col" class="px-3 py-1 text-right">Peso</th>
								</tr>
							</thead>
							<tbody>
								{#each pickRows as r, i (r.ticker)}
									<tr
										class={cn(
											'border-b border-brand-border',
											i % 2 ? 'bg-brand-card' : 'bg-brand-bg'
										)}
									>
										<td class="px-3 py-1 text-right tabular-nums">{r.cluster}</td>
										<td class="px-3 py-1 font-num">{r.ticker}</td>
										<td class={cn('px-3 py-1 text-right tabular-nums', scoreClass(r.score))}>
											{fmtScore(r.score)}
										</td>
										<td class="px-3 py-1 text-right tabular-nums">{fmtRet(r.prior)}</td>
										<td class="px-3 py-1 text-right tabular-nums">{fmtRet(r.view)}</td>
										<td class={cn('px-3 py-1 text-right tabular-nums', signedClassFor(r.post))}>
											{fmtRet(r.post)}
										</td>
										<td class="px-3 py-1 text-right font-num tabular-nums">{fmtShare(r.w)}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				</section>
			</div>
		{/if}
		<p class="mt-3 text-xs text-brand-muted">
			<b>Portafolios precalculados</b> con los parámetros por defecto ({precalc.n_assets} activos, peso
			máx. {fmtShare(precalc.max_weight, 0)}, tasa libre de riesgo {fmtShare(
				precalc.risk_free_rate,
				1
			)}) sobre la corrida {precalc.run_id}. Otro universo o views requiere el sitio con servidor.
		</p>
	{/if}
</Section>
