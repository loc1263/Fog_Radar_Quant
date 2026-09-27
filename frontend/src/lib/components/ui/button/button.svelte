<script lang="ts" module>
	import { cn, type WithElementRef } from '$lib/utils.js';
	import type { HTMLAnchorAttributes, HTMLButtonAttributes } from 'svelte/elements';
	import { type VariantProps, tv } from 'tailwind-variants';

	export const buttonVariants = tv({
		// Dashboard ejecutivo §5.10: sin sombras, bordes finos, texto en bold.
		base: "focus-visible:ring-brand-blue/50 inline-flex shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded text-sm font-bold outline-none transition-colors focus-visible:ring-[3px] disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50 [&_svg:not([class*='size-'])]:size-4 [&_svg]:pointer-events-none [&_svg]:shrink-0",
		variants: {
			variant: {
				// Botón secundario: el estilo por defecto del sistema.
				default: 'border border-brand-border bg-brand-bg text-brand-label hover:bg-brand-card',
				primary: 'border border-brand-strong bg-brand-strong text-white hover:opacity-90',
				destructive: 'border border-brand-red-solid bg-brand-red-solid text-white hover:opacity-90',
				ghost: 'text-brand-label hover:bg-brand-card hover:text-brand-accent',
				link: 'text-brand-label underline-offset-4 hover:underline'
			},
			size: {
				default: 'px-3 py-1.5',
				sm: 'px-2.5 py-1 text-xs',
				lg: 'px-4 py-2',
				icon: 'size-8'
			}
		},
		defaultVariants: {
			variant: 'default',
			size: 'default'
		}
	});

	export type ButtonVariant = VariantProps<typeof buttonVariants>['variant'];
	export type ButtonSize = VariantProps<typeof buttonVariants>['size'];

	export type ButtonProps = WithElementRef<HTMLButtonAttributes> &
		WithElementRef<HTMLAnchorAttributes> & {
			variant?: ButtonVariant;
			size?: ButtonSize;
		};
</script>

<script lang="ts">
	let {
		class: className,
		variant = 'default',
		size = 'default',
		ref = $bindable(null),
		href = undefined,
		type = 'button',
		disabled,
		children,
		...restProps
	}: ButtonProps = $props();
</script>

{#if href}
	<a
		bind:this={ref}
		data-slot="button"
		class={cn(buttonVariants({ variant, size }), className)}
		href={disabled ? undefined : href}
		aria-disabled={disabled}
		role={disabled ? 'link' : undefined}
		tabindex={disabled ? -1 : undefined}
		{...restProps}
	>
		{@render children?.()}
	</a>
{:else}
	<button
		bind:this={ref}
		data-slot="button"
		class={cn(buttonVariants({ variant, size }), className)}
		{type}
		{disabled}
		{...restProps}
	>
		{@render children?.()}
	</button>
{/if}
