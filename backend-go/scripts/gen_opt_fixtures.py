"""Genera internal/quant/testdata/opt.json: max-Sharpe (SLSQP del original), linkage/fcluster de scipy y
cov/corr de pandas sobre retornos sintéticos. Es el oráculo de los tests de optimización de quant.
Uso (desde backend/): uv run python ../backend-go/scripts/gen_opt_fixtures.py
"""

import json
from pathlib import Path

import numpy as np
import pandas as pd
from scipy.cluster.hierarchy import fcluster, linkage
from scipy.spatial.distance import squareform

from fog_radar.modules.us_stocks import portfolio as p

rng = np.random.default_rng(11)
cases = []
for n, t, cap in ((8, 252, 0.35), (12, 250, 0.25), (15, 250, 1.0), (40, 252, 0.10), (60, 252, 0.05)):
    common = rng.normal(0, 0.01, size=(t, 1))
    r = common * rng.uniform(0.3, 1.5, size=(1, n)) + rng.normal(0, 0.012, size=(t, n)) + rng.uniform(-0.0002, 0.0016, size=n)
    df = pd.DataFrame(r, columns=[f"T{i}" for i in range(n)])
    mean = p._annualized_mean(df)
    cov = p._annualized_cov(df)
    w = p._max_sharpe_weights(mean, cov, 0.04, cap)
    corr = df.corr().clip(-1.0, 1.0).fillna(0.0).to_numpy()
    dist = np.sqrt(0.5 * (1.0 - corr))
    np.fill_diagonal(dist, 0.0)
    link = linkage(squareform(dist, checks=False), method="average")
    labels = {str(k): fcluster(link, t=k, criterion="maxclust").tolist() for k in (2, 5, 10) if k < n}
    cases.append(
        {
            "returns": df.to_numpy().T.tolist(),  # columnas
            "cov_sample": df.cov().to_numpy().tolist(),
            "corr": df.corr().to_numpy().tolist(),
            "mean_ann": mean.tolist(),
            "cov_ann": cov.tolist(),
            "rf": 0.04,
            "cap": cap,
            "w": w.tolist(),
            "labels": labels,
            "link_heights": link[:, 2].tolist(),
        }
    )
dest = Path(__file__).resolve().parents[1] / "internal" / "quant" / "testdata" / "opt.json"
dest.write_text(json.dumps(cases), encoding="utf-8")
print(len(cases), "casos ->", dest)
