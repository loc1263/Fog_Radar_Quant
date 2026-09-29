<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { ThemeToggle } from '$lib/components/shared';
	import { EXTRA_NAV, MODULES, isActive, keptQuery, type NavItem } from '$lib/nav';
	import { ruta } from '$lib/nav-href';
	import { cn } from '$lib/utils';

	let { children } = $props();

	const home: NavItem = { label: 'Inicio', href: '/' };
	const current = $derived(MODULES.find((m) => isActive(page.url.pathname, m.href)));
	const subtitle = $derived(
		current
			? current.description
			: 'Análisis cuantitativo de ETFs y acciones de Chile, EEUU y Asia.'
	);

	const LINK = 'rounded px-2.5 py-1 text-sm font-bold';
	const IDLE = 'text-brand-label hover:bg-brand-card hover:text-brand-accent';
	const ACTIVE = 'bg-brand-strong text-white';
</script>

<svelte:head>
	<title>{current ? `${current.label} · Fog Radar Quant` : 'Fog Radar Quant'}</title>
</svelte:head>

<div class="flex min-h-screen flex-col">
	<header class="border-b border-brand-border bg-brand-amber-bg px-7 py-5">
		<div class="flex flex-wrap items-start justify-between gap-3">
			<div>
				<h1 class="text-2xl font-bold">
					<a href={resolve('/')} class="hover:text-brand-accent">Fog Radar Quant</a>
				</h1>
				<div class="mt-1 text-sm text-brand-label">{subtitle}</div>
			</div>
			<ThemeToggle />
		</div>
	</header>

	<nav
		class="sticky top-0 z-10 border-b border-brand-border bg-brand-bg px-7 py-2"
		aria-label="Navegación principal"
	>
		<div class="flex flex-wrap items-center gap-1">
			{#each [home, ...MODULES] as item (item.href)}
				{@const active = isActive(page.url.pathname, item.href)}
				<a
					href={resolve(ruta(item.href))}
					class={cn(LINK, active ? ACTIVE : IDLE)}
					aria-current={active ? 'page' : undefined}>{item.label}</a
				>
			{/each}
			<span class="mx-1 h-5 w-px bg-brand-border" aria-hidden="true"></span>
			{#each EXTRA_NAV as item (item.href)}
				{@const active = isActive(page.url.pathname, item.href)}
				<a
					href={resolve(ruta(item.href))}
					class={cn(LINK, active ? ACTIVE : IDLE)}
					aria-current={active ? 'page' : undefined}>{item.label}</a
				>
			{/each}
			<form method="GET" action={resolve('/buscar')} class="ml-auto" role="search">
				<label class="sr-only" for="buscar-global">Buscar ticker en los 4 módulos</label>
				<input
					id="buscar-global"
					type="search"
					name="q"
					required
					maxlength="40"
					placeholder="Buscar ticker…"
					class="w-44 rounded border border-brand-border bg-brand-bg px-2 py-1 text-sm"
				/>
			</form>
		</div>
		{#if current && current.sections.length > 0}
			<div
				class="mt-2 flex flex-wrap items-center gap-1.5 border-t border-brand-border pt-2"
				role="group"
				aria-label="Secciones de {current.label}"
			>
				{#each current.sections as section (section.href)}
					{@const active = page.url.pathname === section.href}
					<a
						href="{resolve(ruta(section.href))}{keptQuery(page.url, current.keepParams)}"
						aria-current={active ? 'page' : undefined}
						class={cn(
							'rounded border px-2.5 py-0.5 text-xs font-bold',
							active
								? 'border-brand-strong bg-brand-strong text-white'
								: 'border-brand-border bg-brand-bg text-brand-label hover:bg-brand-card'
						)}>{section.label}</a
					>
				{/each}
			</div>
		{/if}
	</nav>

	<main class="mx-auto w-full max-w-[1600px] flex-1 px-7 pb-16">
		{@render children()}
	</main>

	<footer class="border-t border-brand-border py-4 text-center text-sm text-brand-label italic">
		Fog Radar Quant · Información generada automáticamente con fines informativos. No constituye
		asesoría financiera.
	</footer>
</div>
