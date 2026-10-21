// Tema antes de pintar (docs/DESIGN_SYSTEM.md §6): evita el destello en claro.
try {
	var t = localStorage.getItem('fog-radar-theme');
	if (t !== 'light' && t !== 'dark') {
		t = matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
	}
	if (t === 'dark') document.documentElement.classList.add('dark');
} catch {
	// sin localStorage: se queda el tema del sistema
}
