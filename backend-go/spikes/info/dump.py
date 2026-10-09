"""Vuelca yfinance .info de unos tickers a out_py.json (campos que usan los módulos)."""
import json, sys, yfinance as yf
T = sys.argv[1:] or ["AAPL","MSFT","SPY","VTI","COPEC.SN","SQM-B.SN","ENELAM.SN","7203.T","VOO"]
out = {}
for t in T:
    try: out[t] = yf.Ticker(t).info
    except Exception as e: out[t] = {"_error": str(e)}
json.dump(out, open("out_py.json","w"), default=str)
print(len(out))
