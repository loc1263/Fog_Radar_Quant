#!/usr/bin/env python3
"""Levanta backend (FastAPI) y frontend (SvelteKit) juntos para desarrollo.

Uso (desde la raíz del repo, o doble clic en iniciar.bat en Windows):

    python iniciar_dev.py
    python iniciar_dev.py --backend-port 8001 --sin-reload
    python iniciar_dev.py --sin-instalar     # no instala dependencias faltantes

Salida combinada con prefijos [BACKEND]/[FRONTEND]; Ctrl+C detiene ambos.
La primera vez instala lo que falte: `uv sync` en backend/ y `npm install` en frontend/.
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

ROOT = Path(__file__).resolve().parent
BACKEND = ROOT / "backend"
FRONTEND = ROOT / "frontend"
IS_WINDOWS = os.name == "nt"


def _pump(proc: subprocess.Popen[str], prefix: str) -> None:
    assert proc.stdout is not None
    for line in proc.stdout:
        sys.stdout.write(f"{prefix} {line}")
        sys.stdout.flush()


def _check_tools() -> list[str]:
    problemas = []
    if shutil.which("uv") is None:
        problemas.append("No se encontró 'uv'. Instalar: https://docs.astral.sh/uv/getting-started/installation/")
    if shutil.which("npm") is None:
        problemas.append("No se encontró 'npm'. Instalar Node.js 22 o superior: https://nodejs.org/")
    return problemas


def _install_missing() -> bool:
    """Instala dependencias si faltan. Devuelve False si algo falló."""
    pasos = []
    if not (BACKEND / ".venv").is_dir():
        pasos.append(("backend", ["uv", "sync"], BACKEND))
    if not (FRONTEND / "node_modules").is_dir():
        pasos.append(("frontend", ["npm", "install"], FRONTEND))
    for nombre, cmd, cwd in pasos:
        print(f"[SETUP] Instalando dependencias del {nombre}: {' '.join(cmd)} (solo la primera vez)...")
        if subprocess.call(cmd, cwd=cwd, shell=IS_WINDOWS) != 0:
            print(f"[ERROR] Falló '{' '.join(cmd)}' en {cwd}.", file=sys.stderr)
            return False
    return True


def main() -> int:
    # La consola de Windows puede no ser UTF-8: reemplazar en vez de fallar al reenviar salida.
    for stream in (sys.stdout, sys.stderr):
        reconfigure = getattr(stream, "reconfigure", None)
        if reconfigure is not None:
            reconfigure(errors="replace")
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--backend-port", type=int, default=8000)
    parser.add_argument("--frontend-port", type=int, default=5173)
    parser.add_argument("--sin-reload", action="store_true", help="uvicorn sin autorecarga")
    parser.add_argument("--sin-instalar", action="store_true", help="no instalar dependencias faltantes")
    args = parser.parse_args()

    problemas = _check_tools()
    if problemas:
        for p in problemas:
            print(f"[ERROR] {p}", file=sys.stderr)
        return 1
    if args.sin_instalar:
        if not (FRONTEND / "node_modules").is_dir() or not (BACKEND / ".venv").is_dir():
            print("[ERROR] Faltan dependencias: corre sin --sin-instalar o instala a mano.", file=sys.stderr)
            return 1
    elif not _install_missing():
        return 1

    backend_cmd = ["uv", "run", "uvicorn", "fog_radar.main:app", "--port", str(args.backend_port)]
    if not args.sin_reload:
        backend_cmd.append("--reload")
    frontend_cmd = ["npm", "run", "dev", "--", "--port", str(args.frontend_port)]

    # PYTHONUTF8: sin esto, en Windows el stdout redirigido del backend usa cp1252 y el
    # logging falla con caracteres como '≥' o '→'. UV_LINK_MODE: el caché de uv (C:) y el
    # repo (D:) están en discos distintos; se evita el aviso "Failed to hardlink files".
    env_comun = {**os.environ, "PYTHONUTF8": "1", "PYTHONIOENCODING": "utf-8", "UV_LINK_MODE": "copy"}
    env_back = env_comun
    env_front = {**env_comun, "PRIVATE_API_URL": f"http://localhost:{args.backend_port}"}
    popen_kw: dict = {
        "stdout": subprocess.PIPE,
        "stderr": subprocess.STDOUT,
        "text": True,
        "encoding": "utf-8",
        "errors": "replace",
        "bufsize": 1,
    }
    if IS_WINDOWS:
        popen_kw["creationflags"] = subprocess.CREATE_NEW_PROCESS_GROUP  # type: ignore[attr-defined]

    procs = [
        (subprocess.Popen(backend_cmd, cwd=BACKEND, env=env_back, shell=IS_WINDOWS, **popen_kw), "[BACKEND] "),
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
