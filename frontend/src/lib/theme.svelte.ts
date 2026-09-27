/** Tema claro/oscuro (docs/DESIGN_SYSTEM.md §6). El script de app.html lo aplica antes de pintar. */
const KEY = 'fog-radar-theme';
type Theme = 'light' | 'dark';

function stored(): Theme | null {
	try {
		const t = localStorage.getItem(KEY);
		return t === 'light' || t === 'dark' ? t : null;
	} catch {
		return null;
	}
}

class ThemeState {
	current = $state<Theme>('light');

	/** Llamar una vez desde el +layout.svelte raíz (onMount). */
	init() {
		const media = window.matchMedia('(prefers-color-scheme: dark)');
		this.current = stored() ?? (media.matches ? 'dark' : 'light');
		media.addEventListener('change', (e) => {
			if (!stored()) this.set(e.matches ? 'dark' : 'light', false);
		});
	}

	set(t: Theme, persist = true) {
		this.current = t;
		document.documentElement.classList.toggle('dark', t === 'dark');
		if (persist) {
			try {
				localStorage.setItem(KEY, t);
			} catch {
				/* modo privado: se ignora */
			}
		}
	}

	toggle() {
		this.set(this.current === 'dark' ? 'light' : 'dark');
	}
}

export const theme = new ThemeState();
