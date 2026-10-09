"""Genera internal/modules/us/fallback_gen.go desde FALLBACK_TICKERS de universe.py.
Uso (desde backend/): uv run python ../backend-go/scripts/gen_us_fallback.py
"""

import json
from pathlib import Path

from fog_radar.modules.us_stocks.universe import FALLBACK_TICKERS

lines = [
    "// Código generado desde modules/us_stocks/universe.py (scripts/gen_us_fallback.py): no editar a mano.",
    "",
    "package us",
    "",
    "// FallbackTickers es la lista estática de respaldo (large/mega caps de EEUU) si Wikipedia no responde.",
    "var FallbackTickers = []string{",
]
lines += [f"\t{json.dumps(t)}," for t in FALLBACK_TICKERS]
lines.append("}")
out = Path(__file__).resolve().parents[1] / "internal" / "modules" / "us" / "fallback_gen.go"
out.write_text("\n".join(lines) + "\n", encoding="utf-8")
print(len(FALLBACK_TICKERS), "tickers ->", out)
