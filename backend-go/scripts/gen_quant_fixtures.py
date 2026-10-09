"""Genera internal/quant/testdata/stats.json con salidas de pandas/numpy (oráculo de los tests de quant).
Uso (desde backend/): uv run python ../backend-go/scripts/gen_quant_fixtures.py
"""

import json
import random
from pathlib import Path

import numpy as np
import pandas as pd

rng = random.Random(42)
cases = []
for n in (1, 2, 3, 5, 7, 8, 9, 15, 30, 64, 130, 300):
    for kind in ("normal", "heavy", "ints"):
        vals = []
        for _ in range(n):
            if kind == "normal":
                v = rng.gauss(0, 1)
            elif kind == "heavy":
                v = rng.gauss(0, 1) * 10 ** rng.randint(-6, 8)
            else:
                v = float(rng.randint(-5, 5))
            vals.append(v)
        s = pd.Series(vals, dtype=float)
        # con algunos faltantes
        s_nan = s.copy()
        for i in range(0, n, 4):
            s_nan.iloc[i] = np.nan
        for tag, ser in (("full", s), ("nan", s_nan)):
            v = ser.dropna()
            lo, hi = (v.quantile([0.05, 0.95]).tolist() if len(v) else (None, None))
            if len(v) < 5:
                wins = ser.copy()
            else:
                wins = ser.clip(lower=lo, upper=hi)
            z = (ser - v.mean()) / v.std(ddof=0) if len(v) >= 2 and v.std(ddof=0) != 0 else pd.Series(np.nan, index=ser.index)
            cases.append(
                {
                    "values": [None if np.isnan(x) else x for x in ser.tolist()],
                    "mean": None if len(v) == 0 else float(v.mean()),
                    "std0": None if len(v) == 0 else float(v.std(ddof=0)),
                    "q": {str(q): (None if len(v) == 0 else float(v.quantile(q))) for q in (0.05, 0.25, 0.5, 0.95, 1.0)},
                    "winsor": [None if np.isnan(x) else x for x in wins.tolist()],
                    "z": [None if np.isnan(x) else x for x in z.tolist()],
                    "rank": [None if np.isnan(x) else x for x in ser.rank(ascending=False, method="min").tolist()],
                }
            )

reprs = [0.0, -0.0, 1.0, 0.1, 1 / 3, 1e-4, 9.99e-5, 1e-5, 1.5e-7, 123456789.0, 1e15, 1e16, 1.5e16, 1e22, 12345678901234567890.0,
         -2.5, 0.30000000000000004, 5e-324, 1.7976931348623157e308, 100.0, 1234567.0, 0.0001234, 61.25, 0.7148]
for _ in range(200):
    reprs.append(rng.gauss(0, 1) * 10 ** rng.randint(-10, 20))
rounds = [{"x": x, "r4": round(x, 4) if abs(x) < 1e12 else x} for x in [rng.gauss(0, 3) for _ in range(300)] + [0.00005, 0.00015, 2.5e-5, 1.00005, -1.00005]]
rounds = [{"x": r["x"], "r4": float(np.round(r["x"], 4))} for r in rounds]
out = {"cases": cases, "reprs": [{"x": x, "repr": repr(x)} for x in reprs], "rounds": rounds}
dest = Path(__file__).resolve().parents[1] / "internal" / "quant" / "testdata"
dest.mkdir(parents=True, exist_ok=True)
(dest / "stats.json").write_text(json.dumps(out), encoding="utf-8")
print(len(cases), "casos,", len(reprs), "repr,", len(rounds), "round")
