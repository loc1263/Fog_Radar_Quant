#!/usr/bin/env python3
"""Levanta backend (FastAPI) y frontend (SvelteKit) juntos para desarrollo.

Uso (desde la raíz del repo):

    python scripts/iniciar_dev.py
    python scripts/iniciar_dev.py --backend-port 8001 --sin-reload

Salida combinada con prefijos [BACKEND]/[FRONTEND]; Ctrl+C detiene ambos.
Requiere haber corrido una vez `uv sync` en backend/ y `npm install` en frontend/.
"""

from __future__ import annotations

import argparse
import os
import shutil
import signal
import subprocess
import sys
import threading
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BACKEND = ROOT / "backend"
FRONTEND = ROOT / "frontend"
IS_WINDOWS = os.name == "nt"


def _pump(proc: subprocess.Popen[str], prefix: str) -> None:
    assert proc.stdout is not None
    for line in proc.stdout:
        sys.stdout.write(f"{prefix} {line}")
        sys.stdout.flush()


def _check_prereqs() -> list[str]:
    problemas = []
    if shutil.which("uv") is None:
        problemas.append("No se encontró 'uv' (https://docs.astral.sh/uv/).")
    if shutil.which("npm") is None:
        problemas.append("No se encontró 'npm' (Node.js 22+).")
    if not (FRONTEND / "node_modules").is_dir():
        problemas.append("Falta frontend/node_modules: corre 'npm install' en frontend/.")
    return problemas


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--backend-port", type=int, default=8000)
    parser.add_argument("--frontend-port", type=int, default=5173)
    parser.add_argument("--sin-reload", action="store_true", help="uvicorn sin autorecarga")
    args = parser.parse_args()

    problemas = _check_prereqs()
    if problemas:
        for p in problemas:
            print(f"[ERROR] {p}", file=sys.stderr)
        return 1

    backend_cmd = ["uv", "run", "uvicorn", "fog_radar.main:app", "--port", str(args.backend_port)]
    if not args.sin_reload:
        backend_cmd.append("--reload")
    frontend_cmd = ["npm", "run", "dev", "--", "--port", str(args.frontend_port)]

    env_front = {**os.environ, "PRIVATE_API_URL": f"http://localhost:{args.backend_port}"}
    popen_kw: dict = {"stdout": subprocess.PIPE, "stderr": subprocess.STDOUT, "text": True, "bufsize": 1}
    if IS_WINDOWS:
        popen_kw["creationflags"] = subprocess.CREATE_NEW_PROCESS_GROUP  # type: ignore[attr-defined]

    procs = [
        (subprocess.Popen(backend_cmd, cwd=BACKEND, shell=IS_WINDOWS, **popen_kw), "[BACKEND] "),
        (subprocess.Popen(frontend_cmd, cwd=FRONTEND, env=env_front, shell=IS_WINDOWS, **popen_kw), "[FRONTEND]"),
    ]
    for proc, prefix in procs:
        threading.Thread(target=_pump, args=(proc, prefix), daemon=True).start()

    print(f"Backend  → http://localhost:{args.backend_port}/docs")
    print(f"Frontend → http://localhost:{args.frontend_port}  (Ctrl+C para detener)")

    try:
        while all(p.poll() is None for p, _ in procs):
            threading.Event().wait(0.5)
    except KeyboardInterrupt:
        pass
    finally:
        for proc, _ in procs:
            if proc.poll() is None:
                if IS_WINDOWS:
                    proc.send_signal(signal.CTRL_BREAK_EVENT)  # type: ignore[attr-defined]
                else:
                    proc.terminate()
        for proc, _ in procs:
            try:
                proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                proc.kill()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
