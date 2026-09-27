<!--
  Selección del universo + optimización + búsqueda de la mejor combinación.
  Un solo <form> con dos botones (formaction): funciona sin JavaScript y, con
  JS, `use:enhance` conserva la selección y los resultados de ambas acciones.
-->
<script lang="ts">
	import { enhance } from '$app/forms';
	import type { SubmitFunction } from '@sveltejs/kit';
	import type {
		UsBestCombinationResult,
		UsCandidateRow,
		UsPortfolioLimits,
		UsPortfolioResult,
		UsRankBy
	} from '$lib/api/types';
	import { ScatterChart } from '$lib/charts';
	import { KpiCard, KpiGrid, Notice, SectionHeader } from '$lib/components/shared';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { fmtNumber, signedClass as signedClassFor } from '$lib/format';
	import { WeightList } from '$lib/modules/us';
	import {
		combinationsCount,
		fmtRet,
		fmtScore,
		fmtShare,
		fmtUsd,
		scoreClass
	} from '$lib/modules/us/format';
	import { DEFAULT_SELECTION, RANK_LABELS, type MethodInfo } from '$lib/modules/us/portfolio';
	import { cn } from '$lib/utils';
	import type { ActionData } from './$types';

	interface Props {
		method: MethodInfo;
		rows: UsCandidateRow[];
		runId: string;
		limits: UsPortfolioLimits;
		form: ActionData;
	}

	let { method, rows, runId, limits, form }: Props = $props();

	const isBL = $derived(method.api === 'black_litterman');
	const byMomentum = $derived(rows.toSorted((a, b) => b.momentum_12_1 - a.momentum_12_1));
	const defaultSelection = $derived(byMomentum.slice(0, DEFAULT_SELECTION).map((r) => r.ticker));

	// null = selección inicial (top 15 por momentum); luego, lo que marque el usuario.
	let userSelection = $state<string[] | null>(null);
	const selected = $derived(new Set(userSelection ?? defaultSelection));
	const n = $derived(selected.size);

	let viewInputs = $state<Record<string, string>>({});
	// 4 y no 5 (original): con el top 15, C(15,5) = 3.003 supera el tope de 2.500.
	let k = $state(4);
	let rankBy = $state<UsRankBy>('expected_return');
	let pending = $state<'optimizar' | 'combinacion' | null>(null);

	// Resultados recibidos con JS (el `form` de SvelteKit solo guarda el último).
	let optimizacion = $state<UsPortfolioResult | null>(null);
	let combinacion = $state<UsBestCombinationResult | null>(null);
	let errores = $state<{ optimizar?: string; combinacion?: string }>({});

	// Sin JS, la página recarga con `form`: se muestra directamente.
	const opt = $derived(
		optimizacion ?? (form?.accion === 'optimizar' ? (form.optimizacion ?? null) : null)
	);
	const combo = $derived(
		combinacion ?? (form?.accion === 'combinacion' ? (form.combinacion ?? null) : null)
	);
	const errorOpt = $derived(
		errores.optimizar ?? (form?.accion === 'optimizar' ? form.error : undefined)
	);
	const errorCombo = $derived(
		errores.combinacion ?? (form?.accion === 'combinacion' ? form.error : undefined)
	);

	const total = $derived(combinationsCount(n, k));
	const kMax = $derived(Math.min(limits.max_combination_size, n));
	const kValid = $derived(k >= limits.min_combination_size && k <= kMax);
	const overCap = $derived(total > limits.max_combinations);

	function toggle(ticker: string) {
		const current = [...selected];
		userSelection = selected.has(ticker)
			? current.filter((t) => t !== ticker)
			: [...current, ticker];
	}

	const submit: SubmitFunction = ({ action }) => {
		const accion = action.search.includes('combinacion') ? 'combinacion' : 'optimizar';
		pending = accion;
		return async ({ result, update }) => {
			pending = null;
			if (result.type === 'success' && result.data) {
				errores = { ...errores, [accion]: undefined };
				if (result.data.optimizacion) optimizacion = result.data.optimizacion as UsPortfolioResult;
				if (result.data.combinacion)
					combinacion = result.data.combinacion as UsBestCombinationResult;
			} else if (result.type === 'failure') {
				errores = { ...errores, [accion]: String(result.data?.error ?? 'Error al optimizar.') };
			}
			await update({ reset: false, invalidateAll: false });
		};
	};

	const frontier = $derived(
		opt?.efficient_frontier?.map((p) => ({ vol: p.volatility, ret: p.return })) ?? []
	);
	const optimum = $derived(
		opt ? [{ vol: opt.metrics.volatility, ret: opt.metrics.expected_return }] : []
	);
	const pct0 = (v: number) => `${fmtNumber(v * 100, 0)} %`;

	const blRows = $derived(
		opt?.views
			? Object.keys(opt.views)
					.map((t) => ({
						t,
						prior: opt.prior_returns?.[t],
						view: opt.views?.[t],
						post: opt.posterior_returns?.[t],
						w: opt.weights[t] ?? 0
					}))
					.toSorted((a, b) => b.w - a.w)
			: []
	);
</script>

<form method="POST" action="?/optimizar" use:enhance={submit} class="space-y-8">
	<div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_380px]">
		<div>
			<div class="mb-2 flex flex-wrap items-center justify-between gap-2">
				<p class="text-sm font-bold text-brand-label">
					Universo — {n} seleccionados de {rows.length} (corrida {runId})
				</p>
				<div class="flex gap-2">
					<Button
						size="sm"
						type="button"
						onclick={() => (userSelection = rows.map((r) => r.ticker))}>Marcar todos</Button
					>
					<Button size="sm" type="button" onclick={() => (userSelection = [])}
						>Desmarcar todos</Button
					>
					<Button size="sm" type="button" onclick={() => (userSelection = null)}>Top 15</Button>
				</div>
			</div>
			<div class="max-h-[28rem] overflow-auto rounded border border-brand-border">
				<table class="w-full border-collapse text-sm">
					<caption class="sr-only">Universo del portafolio</caption>
					<thead class="sticky top-0 z-[1]">
						<tr class="bg-brand-strong text-white">
							<th scope="col" class="w-8 px-2 py-2"><span class="sr-only">Incluir</span></th>
							<th scope="col" class="px-2 py-2 text-left">Ticker</th>
							<th scope="col" class="px-2 py-2 text-right">Market cap</th>
							<th scope="col" class="px-2 py-2 text-right">Momentum 12-1</th>
							<th scope="col" class="px-2 py-2 text-right">Score</th>
							{#if isBL}<th scope="col" class="px-2 py-2 text-right">View (% anual)</th>{/if}
						</tr>
					</thead>
					<tbody>
						{#each byMomentum as row, i (row.ticker)}
							{@const on = selected.has(row.ticker)}
							<tr
								class={cn('border-b border-brand-border', i % 2 ? 'bg-brand-card' : 'bg-brand-bg')}
							>
								<td class="px-2 py-1.5">
									<input
										type="checkbox"
										name="tickers"
										value={row.ticker}
										checked={on}
										onchange={() => toggle(row.ticker)}
										aria-label="Incluir {row.ticker}"
										class="size-3.5 accent-brand-strong"
									/>
								</td>
								<td class={cn('px-2 py-1.5', on ? 'font-bold' : 'text-brand-muted')}
									>{row.ticker}</td
								>
								<td class="px-2 py-1.5 text-right tabular-nums">{fmtUsd(row.market_cap)}</td>
								<td
									class={cn(
										'px-2 py-1.5 text-right tabular-nums',
										signedClassFor(row.momentum_12_1)
									)}
								>
									{fmtRet(row.momentum_12_1)}
								</td>
								<td
									class={cn('px-2 py-1.5 text-right tabular-nums', scoreClass(row.score.overall))}
								>
									{fmtScore(row.score.overall)}
								</td>
								{#if isBL}
									<td class="px-2 py-1 text-right">
										<input
											type="text"
											inputmode="decimal"
											name="view:{row.ticker}"
											value={viewInputs[row.ticker] ?? ''}
											oninput={(e) =>
												(viewInputs = { ...viewInputs, [row.ticker]: e.currentTarget.value })}
											disabled={!on}
											placeholder={opt?.views?.[row.ticker] != null
												? fmtNumber(opt.views[row.ticker] * 100, 1)
												: 'auto'}
											aria-label="View de {row.ticker} (% anual)"
											class="w-20 rounded border border-brand-border bg-brand-bg px-1.5 py-0.5 text-right text-xs tabular-nums disabled:opacity-50"
										/>
									</td>
								{/if}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>

		<div class="space-y-4">
			<Button
				type="submit"
				variant="primary"
				class="w-full"
				disabled={n < limits.min_tickers || pending !== null}
			>
				{pending === 'optimizar' ? 'Optimizando…' : 'Optimizar'}
			</Button>
			{#if n < limits.min_tickers}
				<p class="text-sm text-brand-muted">Selecciona al menos {limits.min_tickers} tickers.</p>
			{/if}
			{#if errorOpt}
				<Notice kind="warning" title="No se pudo optimizar"><p>{errorOpt}</p></Notice>
			{/if}
			{#if opt}
				<KpiGrid>
					<KpiCard
						label="Retorno esperado anual"
						value={fmtRet(opt.metrics.expected_return)}
						signedValue={opt.metrics.expected_return}
					/>
					<KpiCard label="Volatilidad" value={fmtShare(opt.metrics.volatility, 2)} tone="blue" />
					<KpiCard label="Sharpe" value={fmtNumber(opt.metrics.sharpe)} tone="accent" />
				</KpiGrid>
				<div>
					<SectionHeader>Composición</SectionHeader>
					<WeightList weights={opt.weights} />
				</div>
			{/if}
		</div>
	</div>

	{#if opt && method.api === 'markowitz'}
		<section>
			<SectionHeader>Frontera eficiente</SectionHeader>
			<div class="max-w-3xl">
				<ScatterChart
					x="vol"
					y="ret"
					xFormat={pct0}
					yFormat={pct0}
					groups={[
						{
							key: 'frontera',
							label: 'Frontera eficiente',
							data: frontier,
							color: 'var(--color-brand-blue)'
						},
						{
							key: 'optimo',
							label: 'Óptimo (máx. Sharpe)',
							data: optimum,
							color: 'var(--color-brand-accent)'
						}
					]}
				/>
				<p class="mt-1 text-xs text-brand-muted">
					Eje X: volatilidad anualizada · Eje Y: retorno esperado anualizado.
				</p>
			</div>
		</section>
	{/if}

	{#if opt && isBL && blRows.length}
		<section>
			<SectionHeader>Retornos: equilibrio, views y posterior</SectionHeader>
			<div class="max-w-3xl overflow-x-auto rounded border border-brand-border">
				<table class="w-full border-collapse text-sm">
					<caption class="sr-only">Retornos de Black-Litterman por ticker</caption>
					<thead>
						<tr class="bg-brand-strong text-white">
							<th scope="col" class="px-3 py-2 text-left">Ticker</th>
							<th scope="col" class="px-3 py-2 text-right">Equilibrio (π)</th>
							<th scope="col" class="px-3 py-2 text-right">View (Q)</th>
							<th scope="col" class="px-3 py-2 text-right">Posterior</th>
							<th scope="col" class="px-3 py-2 text-right">Peso</th>
						</tr>
					</thead>
					<tbody>
						{#each blRows as r, i (r.t)}
							<tr
								class={cn('border-b border-brand-border', i % 2 ? 'bg-brand-card' : 'bg-brand-bg')}
							>
								<td class="px-3 py-1.5 font-bold">{r.t}</td>
								<td class="px-3 py-1.5 text-right tabular-nums">{fmtRet(r.prior)}</td>
								<td class="px-3 py-1.5 text-right tabular-nums">{fmtRet(r.view)}</td>
								<td class={cn('px-3 py-1.5 text-right tabular-nums', signedClassFor(r.post))}>
									{fmtRet(r.post)}
								</td>
								<td class="px-3 py-1.5 text-right tabular-nums">{fmtShare(r.w)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</section>
	{/if}

	<section class="space-y-4">
		<SectionHeader>Buscar la mejor combinación</SectionHeader>
		<p class="max-w-3xl text-sm text-brand-muted">
			Prueba todas las combinaciones de <strong>k</strong> tickers dentro de los {n} marcados arriba,
			optimiza cada una con {method.label} y muestra las 5 de mejor
			{RANK_LABELS[rankBy].toLowerCase()}.
		</p>
		<div
			class="flex flex-wrap items-end gap-4 rounded border border-brand-border bg-brand-card p-4"
		>
			<label class="block">
				<span class="mb-1 block text-sm font-bold text-brand-label">Tickers por portafolio (k)</span
				>
				<input
					type="number"
					name="k"
					min={limits.min_combination_size}
					max={Math.max(kMax, limits.min_combination_size)}
					bind:value={k}
					class="w-20 rounded border border-brand-border bg-brand-bg px-2 py-1.5 text-sm tabular-nums"
				/>
			</label>
			<fieldset>
				<legend class="mb-1 text-sm font-bold text-brand-label">Ordenar por</legend>
				<div class="flex gap-2 py-1">
					{#each Object.entries(RANK_LABELS) as [key, label] (key)}
						<label
							class={cn(
								'cursor-pointer rounded border px-2.5 py-0.5 text-xs font-bold',
								rankBy === key
									? 'border-brand-strong bg-brand-strong text-white'
									: 'border-brand-border bg-brand-bg text-brand-label hover:bg-brand-card'
							)}
						>
							<input type="radio" name="rank_by" value={key} bind:group={rankBy} class="sr-only" />
							{label}
						</label>
					{/each}
				</div>
			</fieldset>
			<Button
				type="submit"
				formaction="?/combinacion"
				disabled={n < limits.min_tickers || !kValid || overCap || pending !== null}
			>
				{pending === 'combinacion' ? 'Buscando…' : 'Buscar'}
			</Button>
			<span class="pb-1.5 text-sm text-brand-muted tabular-nums">
				{#if n < limits.min_tickers}
					Marca al menos {limits.min_tickers} tickers arriba.
				{:else if !kValid}
					k tiene que estar entre {limits.min_combination_size} y {kMax}.
				{:else}
					{fmtNumber(total, 0)} combinaciones posibles
				{/if}
			</span>
		</div>
		{#if overCap && kValid}
			<Notice kind="warning">
				<p>
					Esa búsqueda tiene {fmtNumber(total, 0)} combinaciones, más que el máximo soportado ({fmtNumber(
						limits.max_combinations,
						0
					)}). Achica el universo o baja k.
				</p>
			</Notice>
		{/if}
		{#if errorCombo}
			<Notice kind="warning" title="No se pudo buscar"><p>{errorCombo}</p></Notice>
		{/if}
		{#if combo}
			<p class="text-sm text-brand-muted">
				{fmtNumber(combo.combinations_evaluated, 0)} combinaciones evaluadas.
			</p>
			<div class="overflow-x-auto rounded border border-brand-border">
				<table class="w-full border-collapse text-sm">
					<caption class="sr-only">Mejores combinaciones</caption>
					<thead>
						<tr class="bg-brand-strong text-white">
							<th scope="col" class="w-10 px-3 py-2 text-right">#</th>
							<th scope="col" class="px-3 py-2 text-left">Tickers (peso)</th>
							<th scope="col" class="px-3 py-2 text-right">Retorno esp.</th>
							<th scope="col" class="px-3 py-2 text-right">Volatilidad</th>
							<th scope="col" class="px-3 py-2 text-right">Sharpe</th>
						</tr>
					</thead>
					<tbody>
						{#each combo.results as c, i (c.tickers.join(','))}
							<tr
								class={cn('border-b border-brand-border', i % 2 ? 'bg-brand-card' : 'bg-brand-bg')}
							>
								<td class="px-3 py-2 text-right tabular-nums">{i + 1}</td>
								<td class="min-w-[12rem] px-3 py-2">
									{#if i === 0}<Badge variant="mixed" class="mr-2">Mejor</Badge>{/if}
									{c.tickers
										.toSorted((a, b) => (c.weights[b] ?? 0) - (c.weights[a] ?? 0))
										.map((t) => `${t} ${fmtNumber((c.weights[t] ?? 0) * 100, 0)} %`)
										.join(' · ')}
								</td>
								<td
									class={cn(
										'px-3 py-2 text-right tabular-nums',
										signedClassFor(c.metrics.expected_return)
									)}
								>
									{fmtRet(c.metrics.expected_return)}
								</td>
								<td class="px-3 py-2 text-right tabular-nums"
									>{fmtShare(c.metrics.volatility, 2)}</td
								>
								<td
									class={cn(
										'px-3 py-2 text-right tabular-nums',
										i === 0 && 'font-bold text-brand-accent'
									)}
								>
									{fmtNumber(c.metrics.sharpe)}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			{#if combo.results[0]}
				<div class="max-w-2xl">
					<SectionHeader>Composición de la mejor combinación</SectionHeader>
					{#if method.api === 'markowitz'}
						<p class="mb-3 text-sm text-brand-muted">
							Markowitz maximiza el Sharpe sin límite de peso por ticker, así que suele concentrar
							casi todo en 1 o 2 activos ("solución de esquina") en vez de repartir entre los {combo.k}.
							Es lo esperado del modelo, no un error.
						</p>
					{/if}
					<WeightList weights={combo.results[0].weights} tickers={combo.results[0].tickers} />
				</div>
			{/if}
		{/if}
	</section>
</form>
