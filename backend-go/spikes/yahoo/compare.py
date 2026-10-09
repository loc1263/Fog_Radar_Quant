"""Spike B: compara out_go.json (gohoo) contra yfinance. Uso: uv run python compare.py"""
import json, time, yfinance as yf

go = {r["ticker"]: r for r in json.load(open("out_go.json", encoding="utf-8"))}
KEYS = ["currency", "exchange", "marketCap", "trailingPE", "forwardPE", "beta", "sector", "industry",
        "country", "dividendYield", "priceToBook", "trailingEps", "fiftyTwoWeekHigh", "longName", "quoteType"]
# yfinance info -> módulo/clave en quoteSummary
def raw(v):
    return v.get("raw") if isinstance(v, dict) else v
def flat(info):
    d = {}
    for mod in info.values():
        if isinstance(mod, dict):
            for k, v in mod.items():
                d.setdefault(k, raw(v))
    return d

miss, diffs, close_bad = {}, [], []
for tk, g in go.items():
    time.sleep(0.5)
    try:
        h = yf.Ticker(tk).history(period="1y", auto_adjust=False)
        yinfo = yf.Ticker(tk).info
    except Exception as e:
        print(tk, "yf error", e); continue
    if len(h):
        yc, nb = float(h["Close"].iloc[-1]), len(h)
        rel = abs(yc - g["last_close"]) / yc
        if rel > 1e-6 or abs(nb - g["bars"]) > 1:
            close_bad.append((tk, yc, g["last_close"], nb, g["bars"]))
    gi = flat(g.get("info") or {})
    for k in KEYS:
        yv, gv = yinfo.get(k), gi.get(k)
        if yv is not None and gv is None:
            miss.setdefault(k, []).append(tk)
        elif isinstance(yv, (int, float)) and isinstance(gv, (int, float)) and yv and abs(yv - gv) / abs(yv) > 0.02:
            diffs.append((tk, k, yv, gv))
print("\nCIERRE/BARRAS distintos:", len(close_bad)); [print("  ", x) for x in close_bad[:15]]
print("\nCLAVES de info presentes en yfinance y ausentes en gohoo:")
for k, v in miss.items(): print(f"  {k}: {len(v)} tickers, p.ej. {v[:4]}")
print("\nVALORES numéricos con dif >2%:", len(diffs)); [print("  ", x) for x in diffs[:20]]
