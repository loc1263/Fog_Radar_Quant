<!--
  Universo (etapa 1) + parámetros del modelo + resultado: selección por cluster y
  pesos Black-Litterman. Funciona sin JavaScript; con JS, `use:enhance` conserva
  la selección y el resultado.
-->
<script lang="ts">
	import { enhance } from '$app/forms';
	import type { SubmitFunction } from '@sveltejs/kit';
	import type { UsCandidateRow, UsPortfolioLimits, UsPortfolioResult } from '$lib/api/types';
	import { KpiCard, KpiGrid, Notice, SectionHeader } from '$lib/components/shared';
	import { Button } from '$lib/components/ui/button';
	import { fmtNumber, signedClass as signedClassFor } from '$lib/format';
	import { WeightList } from '$lib/modules/us';
	import { fmtRet, fmtScore, fmtShare, fmtUsd, scoreClass } from '$lib/modules/us/format';
	import { TOP_UNIVERSES } from '$lib/modules/us/portfolio';
	import { cn } from '$lib/utils';
	import type { ActionData } from './$types';

	interface Props {
		rows: UsCandidateRow[];
		runId: string;
		limits: UsPortfolioLimits;
		form: ActionData;
	}

	let { rows, runId, limits, form }: Props = $props();

	// Etapa 1: el universo se ordena por el score multi-factor.
	const byScore = $derived(
		rows.toSorted((a, b) => (b.score.overall ?? -1) - (a.score.overall ?? -1))
	);

	// null = todos los candidatos; luego, lo que marque el usuario.
	let userSelection = $state<string[] | null>(null);
	const selected = $derived(new Set(userSelection ?? rows.map((r) => r.ticker)));
	const n = $derived(selected.size);

	let viewInputs = $state<Record<string, string>>({});
	let nAssets = $state<number | null>(null);
	let maxWeightPct = $state<number | null>(null);
	const assetsValue = $derived(nAssets ?? limits.default_assets);
	const maxWeightValue = $derived(maxWeightPct ?? limits.default_max_weight * 100);
	let pending = $state(false);

	// Resultado recibido con JS (el `form` de SvelteKit solo guarda el último).
	let optimizacion = $state<UsPortfolioResult | null>(null);
	let errorLocal = $state<string | undefined>();

	// Sin JS, la página recarga con `form`: se muestra directamente.
	const opt = $derived(optimizacion ?? form?.optimizacion ?? null);
	const errorOpt = $derived(errorLocal ?? form?.error);

	function toggle(ticker: string) {
		const current = [...selected];
		userSelection = selected.has(ticker)
			? current.filter((t) => t !== ticker)
			: [...current, ticker];
	}

	const submit: SubmitFunction = () => {
		pending = true;
		return async ({ result, update }) => {
			pending = false;
			if (result.type === 'success' && result.data?.optimizacion) {
				errorLocal = undefined;
				optimizacion = result.data.optimizacion as UsPortfolioResult;
			} else if (result.type === 'failure') {
				errorLocal = String(result.data?.error ?? 'Error al optimizar.');
			}
			await update({ reset: false, invalidateAll: false });
		};
	};

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
					{#each TOP_UNIVERSES as top (top)}
						<Button
							size="sm"
							type="button"
							onclick={() => (userSelection = byScore.slice(0, top).map((r) => r.ticker))}
							>Top {top}</Button
						>
					{/each}
					<Button size="sm" type="button" onclick={() => (userSelection = null)}>Todos</Button>
				</div>
			</div>
			<div class="max-h-[28rem] overflow-auto rounded border border-brand-border">
				<table class="w-full border-collapse text-sm">
					<caption class="sr-only">Universo del portafolio</caption>
					<thead class="sticky top-0 z-[1]">
						<tr class="bg-brand-strong text-white">
							<th scope="col" class="w-8 px-2 py-1"><span class="sr-only">Incluir</span></th>
							<th scope="col" class="px-2 py-1 text-left">Ticker</th>
							<th scope="col" class="px-2 py-1 text-right">Market cap</th>
							<th scope="col" class="px-2 py-1 text-right">Momentum 12-1</th>
							<th scope="col" class="px-2 py-1 text-right">Score</th>
							<th scope="col" class="px-2 py-1 text-right">View (% anual)</th>
						</tr>
					</thead>
					<tbody>
						{#each byScore as row, i (row.ticker)}
							{@const on = selected.has(row.ticker)}
							<tr
								class={cn('border-b border-brand-border', i % 2 ? 'bg-brand-card' : 'bg-brand-bg')}
							>
								<td class="px-2 py-0.5">
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
								<td class={cn('px-2 py-0.5', on ? 'font-bold' : 'text-brand-muted')}
									>{row.ticker}</td
								>
								<td class="px-2 py-0.5 text-right tabular-nums">{fmtUsd(row.market_cap)}</td>
								<td
									class={cn(
										'px-2 py-0.5 text-right tabular-nums',
										signedClassFor(row.momentum_12_1)
									)}
								>
									{fmtRet(row.momentum_12_1)}
								</td>
								<td
									class={cn('px-2 py-0.5 text-right tabular-nums', scoreClass(row.score.overall))}
								>
									{fmtScore(row.score.overall)}
								</td>
								<td class="px-2 py-0 text-right">
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
										class="w-20 rounded border border-brand-border bg-brand-bg px-1.5 py-0 text-right text-xs leading-5 tabular-nums disabled:opacity-50"
									/>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>

		<div class="space-y-4">
			<div class="grid grid-cols-2 gap-3 rounded border border-brand-border bg-brand-card p-3">
				<label class="block">
					<span class="mb-1 block text-sm font-bold text-brand-label">Activos (clusters)</span>
					<input
						type="number"
						name="n_assets"
						min={limits.min_assets}
						max={limits.max_assets}
						value={assetsValue}
						oninput={(e) => (nAssets = e.currentTarget.valueAsNumber)}
						class="w-full rounded border border-brand-border bg-brand-bg px-2 py-1.5 text-sm tabular-nums"
					/>
				</label>
				<label class="block">
					<span class="mb-1 block text-sm font-bold text-brand-label">Peso máx. por activo (%)</span
					>
					<input
						type="number"
						name="max_weight"
						min="5"
						max="100"
						step="1"
						value={maxWeightValue}
						oninput={(e) => (maxWeightPct = e.currentTarget.valueAsNumber)}
						class="w-full rounded border border-brand-border bg-brand-bg px-2 py-1.5 text-sm tabular-nums"
					/>
				</label>
			</div>
			<Button
				type="submit"
				variant="primary"
				class="w-full"
				disabled={n < limits.min_tickers || pending}
			>
				{pending ? 'Optimizando…' : 'Optimizar'}
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

	{#if opt}
		<section>
			<SectionHeader>Selección por cluster y retornos Black-Litterman</SectionHeader>
			<p class="mb-2 max-w-3xl text-sm text-brand-muted">
				{opt.selection.length} activos de un universo de {opt.universe.length}: el de mayor score de
				cada cluster de correlación. Peso máximo por activo: {fmtShare(opt.max_weight, 0)}.
			</p>
			<div class="max-w-4xl overflow-x-auto rounded border border-brand-border">
				<table class="w-full border-collapse text-sm">
					<caption class="sr-only"
						>Selección por cluster y retornos de Black-Litterman por ticker</caption
					>
					<thead>
						<tr class="bg-brand-strong text-white">
							<th scope="col" class="px-3 py-2 text-right">Cluster</th>
							<th scope="col" class="px-3 py-2 text-left">Ticker</th>
							<th scope="col" class="px-3 py-2 text-right">Score</th>
							<th scope="col" class="px-3 py-2 text-right">Equilibrio (π)</th>
							<th scope="col" class="px-3 py-2 text-right">View (Q)</th>
							<th scope="col" class="px-3 py-2 text-right">Posterior</th>
							<th scope="col" class="px-3 py-2 text-right">Peso</th>
						</tr>
					</thead>
					<tbody>
						{#each pickRows as r, i (r.ticker)}
							<tr
								class={cn('border-b border-brand-border', i % 2 ? 'bg-brand-card' : 'bg-brand-bg')}
							>
								<td class="px-3 py-1.5 text-right tabular-nums">{r.cluster}</td>
								<td class="px-3 py-1.5 font-bold">{r.ticker}</td>
								<td class={cn('px-3 py-1.5 text-right tabular-nums', scoreClass(r.score))}>
									{fmtScore(r.score)}
								</td>
								<td class="px-3 py-1.5 text-right tabular-nums">{fmtRet(r.prior)}</td>
								<td class="px-3 py-1.5 text-right tabular-nums">{fmtRet(r.view)}</td>
								<td class={cn('px-3 py-1.5 text-right tabular-nums', signedClassFor(r.post))}>
									{fmtRet(r.post)}
								</td>
								<td class="px-3 py-1.5 text-right font-bold tabular-nums">{fmtShare(r.w)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</section>
	{/if}
</form>
