<script lang="ts">
	import { page } from '$app/state';
	import { ErrorState } from '$lib/components/shared';

	const title = $derived(page.status === 404 ? 'Página no encontrada' : `Error ${page.status}`);
</script>

<ErrorState
	{title}
	message={page.error?.message ?? 'Ocurrió un error inesperado.'}
	code={page.error?.code}
	retry={page.status !== 404}
/>
