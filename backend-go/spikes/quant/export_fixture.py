"""Spike C: exporta (mu, S, rf, restricciones) del fixture real de Tendencia + pesos de PyPortfolioOpt."""
import json, warnings, pandas as pd
from pypfopt import EfficientFrontier, expected_returns, objective_functions, risk_models
from fog_radar.modules.tendencia import config

FX = "../../../backend/tests/tendencia/fixtures"
prices = pd.read_csv(f"{FX}/data/processed/prices_usd.csv.gz", index_col=0, parse_dates=True)
rf = float(open(f"{FX}/data/processed/risk_free_rate.txt").read().strip())
top15 = pd.read_csv(f"{FX}/expected/20260926_01/top15_seleccion.csv")
print("cols top15:", list(top15.columns)[:5], "prices:", prices.shape)

def caso(nombre, px, bounds, gamma):
    mu = expected_returns.mean_historical_return(px, frequency=config.WEEKS_PER_YEAR)
    S = risk_models.sample_cov(px, frequency=config.WEEKS_PER_YEAR)
    ef = EfficientFrontier(mu, S, weight_bounds=bounds)
    ef.add_objective(objective_functions.L2_reg, gamma=gamma)
    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        ef.max_sharpe(risk_free_rate=rf)
    raw = dict(zip(mu.index, map(float, ef.weights)))
    perf = ef.portfolio_performance(risk_free_rate=rf)
    return dict(nombre=nombre, tickers=list(mu.index), mu=mu.tolist(), S=S.values.tolist(), rf=rf,
                lb=bounds[0], ub=bounds[1], gamma=gamma, w_raw=raw, w_clean=dict(ef.clean_weights()),
                perf=list(map(float, perf)))

casos = [caso("universo", prices, (0, config.MAX_WEIGHT_PER_ASSET), config.L2_GAMMA)]
col = top15.columns[0]
tk = [t for t in top15[col] if t in prices.columns]
casos.append(caso("final15", prices[tk], (0, max(0.30, 1 / len(tk))), 0.05))
json.dump(casos, open("fixture.json", "w"))
for c in casos:
    nz = sum(v > 1e-6 for v in c["w_raw"].values())
    print(c["nombre"], "n=", len(c["tickers"]), "activos>0:", nz, "perf:", c["perf"])
