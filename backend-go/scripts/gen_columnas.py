"""Genera internal/modules/cl/columnas_gen.go desde columnas.py de Python.
Uso (desde backend/): uv run python ../backend-go/scripts/gen_columnas.py
"""

import json
from pathlib import Path

from fog_radar.modules.cl_stocks.columnas import COLUMNAS, GRUPOS


def q(s):
    return "nil" if s is None else json.dumps(s, ensure_ascii=False)


def qs(s):
    return json.dumps(s, ensure_ascii=False)


lines = [
    "// Código generado desde modules/cl_stocks/columnas.py (scripts/gen_columnas.py): no editar a mano.",
    "",
    "package cl",
    "",
    "// Grupos son los grupos de columnas del catálogo de indicadores, en orden.",
    "var Grupos = []string{",
]
lines += [f"\t{qs(g)}," for g in GRUPOS]
lines += ["}", "", "// Columnas es el catálogo de indicadores crudos (CSV de indicadores).", "var Columnas = []Columna{"]
for c in COLUMNAS:
    desc = "nil" if c.descripcion is None else f"strp({qs(c.descripcion)})"
    lines.append(
        f"\t{{CSV: {qs(c.csv)}, Key: {qs(c.key)}, Label: {qs(c.label)}, Grupo: {qs(c.grupo)}, "
        f"Unidad: {qs(c.unidad)}, Descripcion: {desc}}},"
    )
lines.append("}")
out = Path(__file__).resolve().parents[1] / "internal" / "modules" / "cl" / "columnas_gen.go"
out.write_text("\n".join(lines) + "\n", encoding="utf-8")
print(len(COLUMNAS), "columnas ->", out)
