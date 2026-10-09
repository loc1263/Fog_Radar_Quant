"""Spike B2: compara out_fund.json (gohoo) con yfinance para los cálculos del extractor de Chile."""
import json, time, yfinance as yf
go = json.load(open("out_fund.json", encoding="utf-8"))
ok = bad = 0
def pares(s):  # serie yfinance (más reciente primero) -> lista de (fecha, valor)
    return [(str(i.date()), float(v)) for i, v in s.dropna().items()]
for tk in go:
    time.sleep(0.4)
    t = yf.Ticker(tk)
    try:
        rev = pares(t.quarterly_income_stmt.loc["Total Revenue"]) if "Total Revenue" in t.quarterly_income_stmt.index else []
        cap = pares(t.quarterly_cashflow.loc["Capital Expenditure"]) if "Capital Expenditure" in t.quarterly_cashflow.index else []
    except Exception as e:
        print(tk, "yf error", e); continue
    g = go[tk]
    def dic(k): return {f: v for f, v in g.get(k, [])}
    grev, gcap = dic("quarterlyTotalRevenue"), dic("quarterlyCapitalExpenditure")
    # indicadores que calcula el extractor
    def growth(p, d):
        # usa el mismo criterio: 5 trimestres más recientes, actual vs hace un año
        s = sorted(p, reverse=True) if isinstance(p, list) else sorted(d.items(), reverse=True)
        return None if len(s) < 5 or not s[4][1] else (s[0][1] - s[4][1]) / abs(s[4][1])
    yg, gg = growth(rev, None), growth(None, grev)
    ycap = sum(v for _, v in cap[:4]) if cap else None
    gcs = sorted(gcap.items(), reverse=True)[:4]
    gcsum = sum(v for _, v in gcs) if gcs else None
    fechas_y = [f for f, _ in rev[:5]]; fechas_g = [f for f, _ in sorted(grev.items(), reverse=True)[:5]]
    same = (yg is None and gg is None) or (yg is not None and gg is not None and abs(yg - gg) < 1e-9)
    capok = (ycap is None and gcsum is None) or (ycap is not None and gcsum is not None and abs(ycap - gcsum) <= 1e-6 * abs(ycap or 1))
    if same and capok: ok += 1
    else:
        bad += 1
        print(f"{tk:14} growth y={yg} g={gg} | capex4 y={ycap} g={gcsum}\n   fechas y={fechas_y}\n   fechas g={fechas_g}")
print(f"\nIDÉNTICOS: {ok} · DISTINTOS: {bad}")
