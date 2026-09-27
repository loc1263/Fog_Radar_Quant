<script lang="ts" module>
	import { type VariantProps, tv } from 'tailwind-variants';

	export const badgeVariants = tv({
		// Dashboard ejecutivo §5.7: pill con texto blanco sobre relleno sólido.
		base: 'inline-flex w-fit shrink-0 items-center justify-center gap-1 overflow-hidden whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-bold text-white [&>svg]:pointer-events-none [&>svg]:size-3',
		variants: {
			variant: {
				positive: 'bg-brand-green-solid',
				negative: 'bg-brand-red-solid',
				mixed: 'bg-brand-accent-solid',
				neutral: 'bg-brand-neutral',
				// Categorías que no son señal (p. ej. Large/Mid/Small-cap).
				category: 'bg-brand-strong',
				outline: 'border border-brand-border bg-brand-bg text-brand-label'
			}
		},
		defaultVariants: {
			variant: 'category'
		}
	});

	export type BadgeVariant = VariantProps<typeof badgeVariants>['variant'];
</script>

<script lang="ts">
	import type { HTMLAnchorAttributes } from 'svelte/elements';
	import { cn, type WithElementRef } from '$lib/utils.js';

	let {
		ref = $bindable(null),
		href,
		class: className,
		variant = 'category',
		children,
		...restProps
	}: WithElementRef<HTMLAnchorAttributes> & {
		variant?: BadgeVariant;
	} = $props();
</script>

<svelte:element
	this={href ? 'a' : 'span'}
	bind:this={ref}
	data-slot="badge"
	{href}
	class={cn(badgeVariants({ variant }), className)}
	{...restProps}
>
	{@render children?.()}
</svelte:element>
