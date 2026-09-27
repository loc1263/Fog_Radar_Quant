// See https://svelte.dev/docs/kit/types#app.d.ts
declare global {
	namespace App {
		interface Error {
			message: string;
			/** Código estable del backend (`not_found`, `no_data`, `rate_limited`…). */
			code?: string;
		}
		interface Locals {
			/** IP del usuario final; se reenvía al backend en X-Forwarded-For. */
			clientIp: string;
		}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}
}

export {};
