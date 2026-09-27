#!/usr/bin/env node
/**
 * Corre el autofixer oficial de Svelte (el mismo del Svelte MCP) sobre todos
 * los .svelte / .svelte.ts de src/, sin necesitar red.
 *
 *   npm run autofix                 # todo src/
 *   npm run autofix -- src/routes   # una carpeta o archivos puntuales
 *
 * Sale con código 1 si hay issues o sugerencias (lo usa CI).
 * Los componentes copiados de shadcn-svelte (src/lib/components/ui) se omiten
 * salvo que se pasen explícitamente.
 */
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { basename, join, relative } from 'node:path';
import { svelteAutofixer } from '@sveltejs/mcp';

const VENDORED = join('src', 'lib', 'components', 'ui');
const args = process.argv.slice(2);
const roots = args.length ? args : ['src'];

function walk(path, out) {
	const st = statSync(path);
	if (st.isDirectory()) {
		for (const name of readdirSync(path)) walk(join(path, name), out);
	} else if (/\.svelte(\.ts|\.js)?$/.test(path)) {
		out.push(path);
	}
	return out;
}

const wantsVendored = roots.some((r) => r.startsWith(VENDORED));
const files = roots
	.flatMap((r) => walk(r, []))
	.filter((f) => wantsVendored || !f.startsWith(VENDORED));
let failed = 0;
for (const file of files) {
	const res = await svelteAutofixer({
		code: readFileSync(file, 'utf8'),
		desired_svelte_version: 5,
		filename: basename(file)
	});
	if (res.issues.length || res.suggestions.length) {
		failed++;
		console.log(`\n✗ ${relative('.', file)}`);
		for (const i of res.issues) console.log(`  issue: ${i}`);
		for (const s of res.suggestions) console.log(`  sugerencia: ${s}`);
	}
}
console.log(
	`\nsvelte-autofixer: ${files.length - failed}/${files.length} archivos sin observaciones.`
);
process.exit(failed ? 1 : 0);
