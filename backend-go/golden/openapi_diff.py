"""Fase 1: compara el OpenAPI de Go con el de FastAPI (solo la «forma» que importa a `npm run gen:api`).

Uso (desde backend/): uv run python ../backend-go/golden/openapi_diff.py [http://127.0.0.1:8000]
Lista (a) rutas de Python que Go aún no sirve (esperado: lo que falta migrar), y (b) para las rutas de
Go: métodos, parámetros, códigos de respuesta y la forma del cuerpo 200 (propiedades, requeridas,
tipos, nulabilidad). Ignora títulos, descripciones y ejemplos. Sale con 1 si hay diferencias en (b).
"""

import json
import sys
import urllib.request

from fog_radar.main import create_app
from fog_radar.settings import get_settings

base = (sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8000").rstrip("/")
py = create_app(get_settings()).openapi()
with urllib.request.urlopen(base + "/openapi.json", timeout=30) as r:
    go = json.loads(r.read())


def deref(spec, node):
    while isinstance(node, dict) and "$ref" in node:
        cur = spec
        for part in node["$ref"].lstrip("#/").split("/"):
            cur = cur[part]
        node = cur
    return node


def shape(spec, node, seen=()):
    """Proyección estructural: tipo(s), nulabilidad, enum, propiedades requeridas/opcionales, items."""
    ref = node.get("$ref") if isinstance(node, dict) else None
    if ref and ref in seen:
        return {"rec": ref.rsplit("/", 1)[-1]}
    node = deref(spec, node)
    if not isinstance(node, dict):
        return node
    seen = (*seen, ref) if ref else seen
    nullable = False
    for key in ("anyOf", "oneOf"):
        if key in node:
            subs = [s for s in node[key] if s.get("type") != "null"]
            nullable = len(subs) != len(node[key])
            if len(subs) == 1:
                out = shape(spec, subs[0], seen)
                return {**out, "nullable": True} if nullable else out
            return {"any": [shape(spec, s, seen) for s in subs], "nullable": nullable}
    t = node.get("type")
    if isinstance(t, list):
        nullable = "null" in t
        t = next((x for x in t if x != "null"), None)
    if node.get("nullable"):
        nullable = True
    out = {"type": t}
    if nullable:
        out["nullable"] = True
    if "enum" in node:
        out["enum"] = sorted(map(str, node["enum"]))
    if "const" in node:
        out["enum"] = [str(node["const"])]
    if node.get("format") == "date-time":
        out["format"] = "date-time"
    if "properties" in node:
        req = set(node.get("required", []))
        out["props"] = {k: shape(spec, v, seen) for k, v in sorted(node["properties"].items())}
        out["required"] = sorted(req)
    if "items" in node:
        out["items"] = shape(spec, node["items"], seen)
    if "prefixItems" in node:  # tupla homogénea (p. ej. tuple[float, float]) ≡ array de ese tipo
        shapes = [shape(spec, x, seen) for x in node["prefixItems"]]
        if all(x == shapes[0] for x in shapes):
            out["items"] = shapes[0]
    if isinstance(node.get("additionalProperties"), dict) and node["additionalProperties"]:  # {} == true (cualquier valor)
        out["values"] = shape(spec, node["additionalProperties"], seen)
    return out


def body200(spec, op):
    resp = op.get("responses", {}).get("200", {})
    sch = resp.get("content", {}).get("application/json", {}).get("schema")
    return shape(spec, sch) if sch else None


def params(spec, op):
    out = {}
    for p in op.get("parameters", []):
        p = deref(spec, p)
        s = shape(spec, p.get("schema", {}))
        out[(p["in"], p["name"])] = (bool(p.get("required")), s.get("type"), s.get("pattern"))
    return out


def diff(a, b, path):
    if a == b:
        return []
    if isinstance(a, dict) and isinstance(b, dict):
        out = []
        for k in sorted(a.keys() | b.keys()):
            if k not in a:
                out.append(f"{path}.{k}: solo en Go -> {json.dumps(b[k])[:80]}")
            elif k not in b:
                out.append(f"{path}.{k}: solo en Python -> {json.dumps(a[k])[:80]}")
            else:
                out += diff(a[k], b[k], f"{path}.{k}")
        return out
    return [f"{path}: {json.dumps(a)[:80]} (py) vs {json.dumps(b)[:80]} (go)"]


faltan = sorted(set(py["paths"]) - set(go["paths"]))
print(f"Python tiene {len(py['paths'])} rutas; Go sirve {len(go['paths'])}. Sin migrar: {len(faltan)}")
extra = sorted(set(go["paths"]) - set(py["paths"]))
fallas = 0
if extra:
    print("Rutas solo en Go (¡no deberían existir!):", extra)
    fallas += len(extra)

for path in sorted(set(go["paths"]) & set(py["paths"])):
    for method in sorted(go["paths"][path].keys() | py["paths"][path].keys()):
        a, b = py["paths"][path].get(method), go["paths"][path].get(method)
        if a is None or b is None:
            print(f"✗ {method.upper()} {path}: método solo en {'Go' if a is None else 'Python'}")
            fallas += 1
            continue
        d = []
        pa, pb = params(py, a), params(go, b)
        for k in sorted(pa.keys() | pb.keys()):
            if pa.get(k) != pb.get(k):
                d.append(f"param {k}: {pa.get(k)} (py) vs {pb.get(k)} (go)")
        codes_a = {c for c in a["responses"] if c != "422"}
        codes_b = {c for c in b["responses"] if c not in ("422", "default")}
        if codes_a != codes_b:
            d.append(f"respuestas: {sorted(codes_a)} (py) vs {sorted(codes_b)} (go)")
        d += diff(body200(py, a), body200(go, b), "200")
        print(("✓" if not d else "✗"), method.upper(), path)
        for line in d[:12]:
            print("     ", line)
        fallas += bool(d)

print(f"\n{fallas} diferencias")
sys.exit(1 if fallas else 0)
