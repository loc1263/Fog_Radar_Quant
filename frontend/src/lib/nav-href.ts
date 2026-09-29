import type { Pathname, RouteId } from '$app/types';

/**
 * Tipo del argumento de `resolve()` para los hrefs del menú (todos estáticos):
 * `href={resolve(ruta(item.href))}` (también sirve con `page.route.id` de una ruta sin parámetros).
 *
 * Con más de 25 rutas en el sitio, TypeScript ya no puede elegir la sobrecarga
 * de `resolve` para una *unión* de rutas (la distribuye en tuplas y se rinde).
 * Una ruta estática se resuelve igual que `'/'` (solo antepone el `base`), así
 * que se fija ese tipo; el valor no cambia.
 */
export function ruta(href: Pathname | RouteId): '/' {
	return href as '/';
}
