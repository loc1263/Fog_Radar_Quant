"""Fase 0: congela las respuestas JSON de todos los GET sin parámetros de ruta del backend Python (el oráculo de
paridad de la migración) y mide la latencia de cada uno. Uso (desde backend/):
    uv run python ../backend-go/golden/capture.py [directorio_salida]
Lee los datos reales de data/ (solo lectura). Los endpoints costosos/de red (POST, evaluate) no se tocan."""
import json, sys, time
from pathlib import Path
from fastapi.testclient import TestClient
from fog_radar.main import create_app
from fog_radar.settings import get_settings

out = Path(sys.argv[1] if len(sys.argv) > 1 else "../backend-go/golden/python")
out.mkdir(parents=True, exist_ok=True)
s = get_settings()
app = create_app(s)
key = s.api_key.get_secret_value() if getattr(s, "api_key", None) else None
headers = {"X-API-Key": key} if key else {}
client = TestClient(app)
spec = app.openapi()
rows = []
for path, ops in sorted(spec["paths"].items()):
    if "get" not in ops or "{" in path:
        continue
    t = time.perf_counter()
    r = client.get(path, headers=headers)
    ms = (time.perf_counter() - t) * 1000
    rows.append((path, r.status_code, round(ms), len(r.content)))
    name = path.strip("/").replace("/", "__") + ".json"
    try:
        (out / name).write_text(json.dumps(r.json(), ensure_ascii=False, indent=1, sort_keys=True), encoding="utf-8")
    except Exception:
        pass
print(f"{'endpoint':55} {'HTTP':>4} {'ms':>6} {'bytes':>9}")
for p, c, ms, b in sorted(rows, key=lambda x: -x[2]):
    print(f"{p:55} {c:>4} {ms:>6} {b:>9}")
ms = sorted(x[2] for x in rows)
print(f"\n{len(rows)} endpoints · mediana {ms[len(ms)//2]} ms · p95 {ms[int(len(ms)*.95)]} ms · máx {ms[-1]} ms")
