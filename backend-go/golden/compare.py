"""Fase 1: compara las respuestas del backend Go con las del backend Python sobre los mismos datos.

Uso (desde backend/, con el servidor Go ya levantado: `go run ./cmd/fogradar api` en backend-go/):
    uv run python ../backend-go/golden/compare.py [http://127.0.0.1:8000]

Python se consulta en proceso (TestClient) y Go por HTTP. Compara status y JSON (números por valor:
5.0 == 5) en los endpoints de sistema y en el historial de Chile (ticker, indicador y diff incluidos).
`proxima` de /jobs/programas depende de la hora: solo se verifica que sea una fecha ISO futura.
Sale con código 1 si hay diferencias.
"""

import json
import sys
import urllib.error
import urllib.request
from urllib.parse import quote
from datetime import datetime

from fastapi.testclient import TestClient

from fog_radar.main import create_app
from fog_radar.settings import get_settings

base = (sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8000").rstrip("/")
settings = get_settings()
key = settings.api_key.get_secret_value() if settings.api_key else None
headers = {"X-API-Key": key} if key else {}
py = TestClient(create_app(settings))


def go_get(path: str, body: object = None) -> tuple[int, object]:
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(base + path, data=data, headers={**headers, "Content-Type": "application/json"}, method="GET" if body is None else "POST")
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            return r.status, json.loads(r.read())
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read() or b"null")


def py_get(path: str, body: object = None) -> tuple[int, object]:
    r = py.get(path, headers=headers) if body is None else py.post(path, json=body, headers=headers)
    return r.status_code, r.json()


def diff(a, b, path="$", tol=1e-12) -> list[str]:
    if isinstance(a, dict) and isinstance(b, dict):
        out = []
        for k in sorted(a.keys() | b.keys()):
            if k not in a:
                out.append(f"{path}.{k}: solo en Go")
            elif k not in b:
                out.append(f"{path}.{k}: solo en Python")
            else:
                out += diff(a[k], b[k], f"{path}.{k}", tol)
        return out
    if isinstance(a, list) and isinstance(b, list):
        if len(a) != len(b):
            return [f"{path}: largo {len(a)} (py) vs {len(b)} (go)"]
        return [d for i, (x, y) in enumerate(zip(a, b)) for d in diff(x, y, f"{path}[{i}]", tol)]
    if isinstance(a, bool) or isinstance(b, bool) or a is None or b is None:
        return [] if a is b else [f"{path}: {a!r} (py) vs {b!r} (go)"]
    if isinstance(a, (int, float)) and isinstance(b, (int, float)):
        # pandas.read_csv parsea floats con un algoritmo rápido que puede errar 1 ulp: tolerancia 1e-12
        ok = a == b or abs(a - b) <= tol * max(abs(a), abs(b), 1e-300 if tol < 1e-6 else 1.0)
        return [] if ok else [f"{path}: {a!r} (py) vs {b!r} (go)"]
    return [] if a == b else [f"{path}: {a!r} (py) vs {b!r} (go)"]


# --- caminos a comparar ---------------------------------------------------------------
paths = ["/api/health", "/api/status", "/api/jobs", "/api/jobs/cl_stocks", "/api/jobs/etf?limit=3",
         "/api/jobs/nope", "/api/jobs/etf?limit=0",
         "/api/cl/status", "/api/us/status", "/api/etf/status", "/api/tendencia/status",
         "/api/cl/history/runs", "/api/cl/history/diff", "/api/cl/ranking", "/api/cl/indicadores"]
_, runs = py_get("/api/cl/history/runs")
if isinstance(runs, list) and len(runs) >= 2:
    a, b = runs[0]["ts"], runs[-1]["ts"]
    paths += [f"/api/cl/history/diff?desde={a}&hasta={b}", f"/api/cl/history/diff?desde={b}&hasta={a}",
              f"/api/cl/history/diff?desde={a}", "/api/cl/history/diff?desde=20000101_000000&hasta=" + b]
    _, ranking = py_get(f"/api/cl/history/diff?desde={a}&hasta={b}")
    tickers = [f["ticker"] for f in ranking["filas"][:5]] + ["NOEXISTE.SN"]
    for t in tickers:
        paths += [f"/api/cl/history/ticker/{t}", f"/api/cl/history/ticker/{t.lower()}"]
    for m in ("market_cap", "ret_1m", "ex_dividend_date", "inexistente"):
        paths.append(f"/api/cl/history/ticker/{tickers[0]}/indicador/{m}")
paths.append("/api/cl/history/ticker/" + "A" * 16)  # patrón inválido → 422 en ambos

# --- Tendencia ---
paths += ["/api/tendencia/latest", "/api/tendencia/excluidos", "/api/tendencia/analyses", "/api/tendencia/runs",
          "/api/tendencia/runs?analysis=_us&limit=3", "/api/tendencia/runs?analysis=zz", "/api/tendencia/history/performance",
          "/api/tendencia/history/performance?from=2026-09-01&to=2026-10-30", "/api/tendencia/history/tickers",
          "/api/tendencia/history/tickers?analysis=_us", "/api/tendencia/history/sectors", "/api/tendencia/history/sectors?analysis=_us",
          "/api/tendencia/history/momentum-leaders", "/api/tendencia/history/momentum-leaders?top=2&analysis=_us",
          "/api/tendencia/history/momentum-leaders?top=99", "/api/tendencia/runs/00000000_00", "/api/tendencia/runs/malo/dashboard",
          "/api/tendencia/history/ticker/ZZZZ", "/api/tendencia/history/universe/ZZZZ"]
_, td_runs = py_get("/api/tendencia/runs")
if isinstance(td_runs, list) and td_runs:
    last = td_runs[-1]["run_dir"]
    paths += [f"/api/tendencia/runs/{last}", f"/api/tendencia/runs/{last}/dashboard", f"/api/tendencia/runs/{last}/files",
              f"/api/tendencia/runs/{td_runs[0]['run_dir']}/dashboard", f"/api/tendencia/runs/{last}/files/nada.pdf",
              f"/api/tendencia/runs/{last}/files/dashboard_data.json"]
    _, ht = py_get("/api/tendencia/history/tickers")
    for t in [x["ticker"] for x in ht[:3]]:
        paths += [f"/api/tendencia/history/ticker/{t}", f"/api/tendencia/history/universe/{t}"]

# --- ETF ---
paths += ["/api/etf/resumen", "/api/etf/top15", "/api/etf/top15-simple", "/api/etf/top15-vanguard", "/api/etf/ranking-completo",
          "/api/etf/excluidos", "/api/etf/diversificacion", "/api/etf/rotacion", "/api/etf/momentum/top15",
          "/api/etf/categorias/ranking", "/api/etf/russell3000", "/api/etf/historial", "/api/etf/history/runs",
          "/api/etf/history/top", "/api/etf/history/top?ultimas=3", "/api/etf/history/top?ultimas=1", "/api/etf/history/diff",
          "/api/etf/history/categorias", "/api/etf/history/ticker/CRAK", "/api/etf/history/ticker/crak", "/api/etf/history/ticker/ZZZZZ",
          "/api/etf/history/diff?desde=20260101", "/api/etf/history/diff?desde=malo&hasta=malo"]
_, etf_runs = py_get("/api/etf/history/runs")
if isinstance(etf_runs, list) and len(etf_runs) >= 2:
    paths.append(f"/api/etf/history/diff?desde={etf_runs[0]['run_key']}&hasta={etf_runs[-1]['run_key']}")

# --- EEUU ---
paths += ["/api/us/rankings/latest", "/api/us/rankings/latest/candidates", "/api/us/rankings/runs",
          "/api/us/rankings/latest/prices?days=5&tickers=AAPL&tickers=nvda", "/api/us/rankings/latest/prices?days=1",
          "/api/us/history/runs", "/api/us/history/top", "/api/us/history/top?ultimas=2", "/api/us/history/top?ultimas=1",
          "/api/us/history/diff", "/api/us/history/diff?desde=2026-01-01T00-00-00Z&hasta=2026-01-02T00-00-00Z",
          "/api/us/history/diff?desde=malo", "/api/us/portfolio/limits", "/api/us/history/ticker/ZZZZ"]
_, us_runs = py_get("/api/us/history/runs")
if isinstance(us_runs, list) and us_runs:
    if len(us_runs) >= 2:
        paths.append(f"/api/us/history/diff?desde={us_runs[0]['run_id']}&hasta={us_runs[-1]['run_id']}")
    _, top = py_get("/api/us/rankings/latest")
    for row in top["rows"][:3]:
        paths.append(f"/api/us/history/ticker/{row['ticker']}")
    tks = [r["ticker"] for r in top["rows"]]
    posts = [
        {"tickers": tks},
        {"tickers": tks, "n_assets": 5, "max_weight": 0.3},
        {"tickers": tks, "n_assets": 4, "max_weight": 0.5, "risk_free_rate": 0.02, "views": {tks[0].lower(): 0.5}},
        {"tickers": tks[:1]},
        {"tickers": ["NOPE1", "NOPE2"]},
        {"tickers": tks, "n_assets": 1},
        {"tickers": tks, "max_weight": 0.01},
    ]

fallas = 0
posts = locals().get("posts", [])
for p in paths + [("POST", "/api/us/portfolio/optimize", b) for b in posts]:
    if isinstance(p, tuple):
        _, ruta, body = p
        (sp, jp), (sg, jg) = py_get(ruta, body), go_get(ruta, body)
        p = f"POST {ruta} {json.dumps(body)[:70]}"
    else:
        (sp, jp), (sg, jg) = py_get(p), go_get(p)
    if sp != sg:
        print(f"✗ {p}: HTTP {sp} (py) vs {sg} (go)")
        fallas += 1
        continue
    if sp == 422:  # el texto de validación difiere entre FastAPI y huma; el contrato es status + code
        ok = jp["code"] == jg["code"] == "validation_error"
        print(("✓" if ok else "✗"), p, "(422: solo status y code)")
        fallas += not ok
        continue
    if p == "/api/jobs/programas":
        continue
    if p == "/api/etf/diversificacion":  # el orden de las categorías empatadas lo decide el sort (SIMD) de pandas: se normaliza
        for j in (jp, jg):
            j["filas"].sort(key=lambda f: (-f["conteo"], f["categoria"]))
    # El óptimo de SLSQP (scipy) es aproximado a ~1e-6; el QP de Go es más exacto: tolerancia 1e-4 en portafolios.
    d = diff(jp, jg, tol=1e-4 if "portfolio/optimize" in p else 1e-12)
    print(("✓" if not d else "✗"), p, f"[{sp}]")
    for line in d[:8]:
        print("    ", line)
    fallas += bool(d)

# programas: todo igual salvo `proxima` (depende de la hora)
_, jp = py_get("/api/jobs/programas")
sg, jg = go_get("/api/jobs/programas")
d = []
for x, y in zip(jp, jg):
    px, py_ = x.pop("proxima"), y.pop("proxima")
    d += diff(x, y)
    for v in (px, py_):
        if datetime.fromisoformat(v) <= datetime.now(datetime.fromisoformat(v).tzinfo):
            d.append(f"proxima en el pasado: {v}")
    if px != py_:
        d.append(f"proxima: {px} (py) vs {py_} (go)")
print(("✓" if not d else "✗"), "/api/jobs/programas", d[:5])
fallas += bool(d)

# hub y buscador: solo el aporte de Chile (los otros módulos aún los sirve Python); sin campos que dependen de la hora
_, hp = py_get("/api/hub")
sg, hg = go_get("/api/hub")
cl_p = next(m for m in hp["modulos"] if m["key"] == "cl_stocks")
cl_g = next(m for m in hg["modulos"] if m["key"] == "cl_stocks")
for m in (cl_p, cl_g):
    for k in ("proxima", "edad_horas"):
        m.pop(k, None)
d = diff(cl_p, cl_g)
print(("✓" if not d else "✗"), "/api/hub (cl_stocks)")
for line in d[:6]:
    print("    ", line)
fallas += bool(d)
for q in ("vapores", "SQM", "banco", "zzz", "cl", "ÁNDINA"):
    _, bp = py_get(f"/api/buscar?q={quote(q)}&limite=50")
    sg, bg = go_get(f"/api/buscar?q={quote(q)}&limite=50")
    keep = lambda b: [r for r in b["resultados"] if r["modulo"] == "cl_stocks"]
    d = diff(keep(bp), keep(bg))
    print(("✓" if not d else "✗"), f"/api/buscar?q={q} (cl_stocks, {len(keep(bp))} resultados)")
    for line in d[:6]:
        print("    ", line)
    fallas += bool(d)

print(f"\n{len(paths) + 1} caminos · {fallas} con diferencias")
sys.exit(1 if fallas else 0)
