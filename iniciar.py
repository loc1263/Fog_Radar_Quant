#!/usr/bin/env python3
"""Levanta el backend Go (:8000) y el frontend SvelteKit (:5173) en modo desarrollo.

Uso:  python iniciar.py [--rebuild] [--no-browser] [--no-kill] [--backend-port N] [--frontend-port N]
      (o doble clic en iniciar.bat)

- Verifica requisitos (Go, Node/npm, dependencias, puertos libres: si están ocupados, cierra el proceso que los usa;
  con --no-kill falla en su lugar).
- Compila el backend si falta el binario o hay fuentes .go más nuevas.
- Exporta los datos del sitio (frontend/static/data) con `fogradar export`.
- Muestra la salida de ambos servidores en una sola consola, con prefijo y color.
- Espera a que respondan, imprime las URLs y vigila que ninguno se caiga.
- Ctrl+C (o la caída de uno de los dos) detiene ambos limpiamente.
"""

from __future__ import annotations

import argparse
import os
import shutil
import socket
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.request
import webbrowser
from pathlib import Path

ROOT = Path(__file__).resolve().parent
BACKEND_DIR = ROOT / "backend-go"
FRONTEND_DIR = ROOT / "frontend"
BACKEND_EXE = BACKEND_DIR / "bin" / ("fogradar.exe" if os.name == "nt" else "fogradar")

for _stream in (sys.stdout, sys.stderr):  # la salida de vite/go trae Unicode; cp1252 no lo soporta
    try:
        _stream.reconfigure(encoding="utf-8", errors="replace")
    except (AttributeError, ValueError):
        pass

IS_WIN = os.name == "nt"
USE_COLOR = sys.stdout.isatty()
if IS_WIN and USE_COLOR:
    os.system("")  # habilita secuencias ANSI en la consola de Windows

COLORS = {"sys": "37", "back": "36", "front": "35", "ok": "32", "warn": "33", "err": "31"}


def c(text: str, color: str) -> str:
    return f"\033[{COLORS[color]}m{text}\033[0m" if USE_COLOR else text


print_lock = threading.Lock()


def log(tag: str, msg: str, color: str | None = None) -> None:
    prefix = c(f"[{tag:<8}]", color or tag)
    with print_lock:
        print(f"{prefix} {msg}", flush=True)


def info(msg: str) -> None:
    log("sistema", msg, "sys")


def ok(msg: str) -> None:
    log("sistema", c(msg, "ok"), "sys")


def warn(msg: str) -> None:
    log("sistema", c(msg, "warn"), "sys")


class StartupError(Exception):
    """Error esperado de arranque: se muestra el mensaje y se sale con código 1."""


# ---------------------------------------------------------------- requisitos


def tool_version(cmd: list[str]) -> str:
    try:
        out = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
    except (OSError, subprocess.TimeoutExpired) as e:
        raise StartupError(f"No se pudo ejecutar {' '.join(cmd)}: {e}") from e
    return (out.stdout or out.stderr).strip().splitlines()[0] if (out.stdout or out.stderr) else "?"


def check_requirements() -> tuple[str, str]:
    """Devuelve (go, npm) resueltos; lanza StartupError si falta algo."""
    missing = []
    go = shutil.which("go")
    npm = shutil.which("npm")
    if not BACKEND_EXE.exists() and not go:
        missing.append("Go (https://go.dev/dl/) — necesario para compilar el backend")
    if not npm:
        missing.append("Node.js / npm (https://nodejs.org/) — necesario para el frontend")
    if missing:
        raise StartupError("Faltan requisitos:\n  - " + "\n  - ".join(missing))
    if not (BACKEND_DIR / "go.mod").exists() or not (FRONTEND_DIR / "package.json").exists():
        raise StartupError(f"No se encuentran backend-go/ o frontend/ en {ROOT}")

    if go:
        info(f"Go:   {tool_version([go, 'version'])}")
    info(f"Node: {tool_version(['node', '--version'])}   npm: {tool_version([npm, '--version'])}")

    if not (FRONTEND_DIR / "node_modules").is_dir():
        warn("Falta frontend/node_modules: ejecutando npm install (puede tardar)...")
        run_step([npm, "install"], FRONTEND_DIR, "npm install")

    if not (FRONTEND_DIR / ".env").exists():
        warn("No existe frontend/.env (se usan los valores por defecto: backend en http://localhost:8000).")
        warn("Cópialo desde frontend/.env.example si necesitas otra URL o API key.")
    if not (BACKEND_DIR / ".env").exists():
        warn("No existe backend-go/.env (se usan los valores por defecto). Ver backend-go/.env.example.")
    return go or "", npm  # type: ignore[return-value]


def run_step(cmd: list[str], cwd: Path, label: str, env: dict[str, str] | None = None) -> None:
    """Ejecuta un paso bloqueante mostrando su salida; lanza StartupError si falla."""
    try:
        proc = subprocess.Popen(
            cmd, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
            text=True, encoding="utf-8", errors="replace",
        )
    except OSError as e:
        raise StartupError(f"{label}: no se pudo lanzar ({e})") from e
    assert proc.stdout is not None
    for line in proc.stdout:
        log("sistema", line.rstrip(), "sys")
    if proc.wait() != 0:
        raise StartupError(f"{label} falló (código {proc.returncode}). Revisa los mensajes de arriba.")


def backend_needs_build(force: bool) -> bool:
    if force or not BACKEND_EXE.exists():
        return True
    built = BACKEND_EXE.stat().st_mtime
    sources = [BACKEND_DIR / "go.mod", BACKEND_DIR / "go.sum"]
    sources += (BACKEND_DIR).rglob("*.go")
    return any(p.exists() and p.stat().st_mtime > built for p in sources)


def build_backend(go: str, force: bool) -> None:
    if not backend_needs_build(force):
        ok("Backend: binario al día.")
        return
    info("Compilando el backend...")
    t0 = time.time()
    env = {**os.environ, "CGO_ENABLED": "0"}
    BACKEND_EXE.parent.mkdir(parents=True, exist_ok=True)
    run_step(
        [go, "build", "-trimpath", "-o", str(BACKEND_EXE), "./cmd/fogradar"],
        BACKEND_DIR, "Compilación del backend", env,
    )
    ok(f"Backend compilado en {time.time() - t0:.1f}s.")


def export_data() -> None:
    """Genera frontend/static/data (JSON de la API) con `fogradar export`; el sitio lo lee sin backend."""
    info("Exportando los datos del sitio (fogradar export)...")
    try:
        run_step([str(BACKEND_EXE), "export", "--out", str(FRONTEND_DIR / "static" / "data")],
                 BACKEND_DIR, "Exportación de datos")
    except StartupError as e:  # no es fatal: el sitio muestra "sin datos" en las vistas que dependen de esto
        warn(f"{e} Las vistas de ETF pueden aparecer vacías.")


# -------------------------------------------------------------------- puertos


def port_in_use(port: int) -> bool:
    return bool(listening_pids(port)) or _connects(port)


def _connects(port: int) -> bool:
    for family, host in ((socket.AF_INET, "127.0.0.1"), (socket.AF_INET6, "::1")):
        try:
            with socket.socket(family, socket.SOCK_STREAM) as s:
                s.settimeout(0.5)
                if s.connect_ex((host, port)) == 0:
                    return True
        except OSError:
            continue
    return False


def listening_pids(port: int) -> set[int]:
    """PIDs que escuchan en `port` (TCP). Vacío si no se puede determinar."""
    pids: set[int] = set()
    try:
        if IS_WIN:
            out = subprocess.run(["netstat", "-ano", "-p", "TCP"], capture_output=True, text=True, timeout=15).stdout
            for line in out.splitlines():
                cols = line.split()
                if len(cols) >= 5 and cols[3].upper() == "LISTENING" and cols[1].rsplit(":", 1)[-1] == str(port):
                    pids.add(int(cols[4]))
        elif shutil.which("lsof"):
            out = subprocess.run(["lsof", "-t", f"-iTCP:{port}", "-sTCP:LISTEN"],
                                 capture_output=True, text=True, timeout=15).stdout
            pids.update(int(x) for x in out.split())
        elif shutil.which("ss"):
            import re
            out = subprocess.run(["ss", "-ltnp", f"sport = :{port}"], capture_output=True, text=True, timeout=15).stdout
            pids.update(int(x) for x in re.findall(r"pid=(\d+)", out))
    except (OSError, subprocess.TimeoutExpired, ValueError):
        pass
    pids.discard(os.getpid())
    return {x for x in pids if x > 4}  # 0 y 4 son procesos del sistema en Windows


def process_name(pid: int) -> str:
    try:
        if IS_WIN:
            out = subprocess.run(["tasklist", "/FI", f"PID eq {pid}", "/FO", "CSV", "/NH"],
                                 capture_output=True, text=True, timeout=15).stdout.strip()
            return out.split('","')[0].strip('"') if out.startswith('"') else "?"
        return subprocess.run(["ps", "-p", str(pid), "-o", "comm="], capture_output=True, text=True,
                              timeout=15).stdout.strip() or "?"
    except (OSError, subprocess.TimeoutExpired):
        return "?"


def kill_pid(pid: int) -> None:
    if IS_WIN:
        subprocess.run(["taskkill", "/PID", str(pid), "/T", "/F"], capture_output=True)
    else:
        import signal
        try:
            os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            pass


def free_port(name: str, port: int) -> bool:
    """Cierra lo que ocupa `port`. Devuelve True si quedó libre."""
    pids = listening_pids(port)
    if not pids:
        return not port_in_use(port)
    for pid in sorted(pids):
        warn(f"Puerto {port} ({name}) ocupado por {process_name(pid)} (PID {pid}): cerrándolo...")
        kill_pid(pid)
    for _ in range(20):  # hasta ~5 s a que el SO libere el puerto
        if not port_in_use(port):
            ok(f"Puerto {port} liberado.")
            return True
        time.sleep(0.25)
    return False


def check_ports(ports: dict[str, int], kill: bool = True) -> None:
    busy = [(name, p) for name, p in ports.items() if port_in_use(p)]
    if busy and kill:
        busy = [(name, p) for name, p in busy if not free_port(name, p)]
    if busy:
        lista = ", ".join(f"{n} (:{p})" for n, p in busy)
        ayuda = ("No se pudieron liberar (¿permisos?). Ciérralos a mano" if kill
                 else "Ciérralos, quita --no-kill") + " o usa --backend-port / --frontend-port."
        raise StartupError(f"Puertos ocupados: {lista}.\n  {ayuda}")


# ------------------------------------------------------------------ procesos


class Service:
    def __init__(self, name: str, tag: str, cmd: list[str], cwd: Path, port: int, health_path: str,
                 env: dict[str, str] | None = None):
        self.name, self.tag, self.cmd, self.cwd = name, tag, cmd, cwd
        self.port, self.health_path, self.env = port, health_path, env
        self.proc: subprocess.Popen[str] | None = None

    @property
    def url(self) -> str:
        return f"http://localhost:{self.port}"

    def start(self) -> None:
        flags = subprocess.CREATE_NEW_PROCESS_GROUP if IS_WIN else 0
        try:
            self.proc = subprocess.Popen(
                self.cmd, cwd=self.cwd, env=self.env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                text=True, encoding="utf-8", errors="replace", creationflags=flags,
                start_new_session=not IS_WIN,
            )
        except OSError as e:
            raise StartupError(f"{self.name}: no se pudo iniciar ({e})") from e
        threading.Thread(target=self._pump, daemon=True).start()

    def _pump(self) -> None:
        assert self.proc and self.proc.stdout
        for line in self.proc.stdout:
            log(self.tag, line.rstrip(), self.tag)

    def alive(self) -> bool:
        return self.proc is not None and self.proc.poll() is None

    def healthy(self) -> bool:
        try:
            with urllib.request.urlopen(self.url + self.health_path, timeout=2) as r:
                return r.status < 500
        except urllib.error.HTTPError as e:
            return e.code < 500
        except (urllib.error.URLError, OSError):
            return False

    def wait_ready(self, timeout: float) -> None:
        t0 = time.time()
        while time.time() - t0 < timeout:
            if not self.alive():
                raise StartupError(f"{self.name} terminó inesperadamente (código {self.proc.returncode}) "  # type: ignore[union-attr]
                                   "antes de responder. Revisa su salida arriba.")
            if self.healthy():
                ok(f"{self.name} listo en {self.url} ({time.time() - t0:.1f}s)")
                return
            time.sleep(0.5)
        raise StartupError(f"{self.name} no respondió en {timeout:.0f}s ({self.url}{self.health_path}).")

    def stop(self) -> None:
        if not self.proc or self.proc.poll() is not None:
            return
        pid = self.proc.pid
        if IS_WIN:  # mata el árbol completo (npm → node → vite)
            subprocess.run(["taskkill", "/PID", str(pid), "/T", "/F"], capture_output=True)
        else:
            import signal
            try:
                os.killpg(os.getpgid(pid), signal.SIGTERM)
            except ProcessLookupError:
                pass
        try:
            self.proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.proc.kill()


def supervise(services: list[Service]) -> int:
    """Bloquea hasta que un servicio muera o el usuario pulse Ctrl+C."""
    while True:
        for s in services:
            if not s.alive():
                code = s.proc.returncode if s.proc else "?"
                log("sistema", c(f"{s.name} se detuvo (código {code}). Deteniendo el resto...", "err"), "sys")
                return 1
        time.sleep(1)


# ---------------------------------------------------------------------- main


def main() -> int:
    ap = argparse.ArgumentParser(description="Levanta backend Go + frontend SvelteKit (dev).")
    ap.add_argument("--rebuild", action="store_true", help="fuerza recompilar el backend")
    ap.add_argument("--no-browser", action="store_true", help="no abrir el navegador")
    ap.add_argument("--backend-port", type=int, default=8000)
    ap.add_argument("--frontend-port", type=int, default=5173)
    ap.add_argument("--no-kill", action="store_true", help="no cerrar procesos que ocupen los puertos (falla en su lugar)")
    ap.add_argument("--timeout", type=float, default=90, help="segundos de espera por servidor")
    args = ap.parse_args()

    print(c("=== Fog Radar Quant — entorno de desarrollo ===", "ok"))
    services: list[Service] = []
    try:
        info(f"Proyecto: {ROOT}")
        go, npm = check_requirements()
        check_ports({"backend": args.backend_port, "frontend": args.frontend_port},
                    kill=not args.no_kill)
        build_backend(go, args.rebuild)
        export_data()

        back_env = {**os.environ, "ADDR": f":{args.backend_port}"}
        back = Service("Backend", "back", [str(BACKEND_EXE), "api"], BACKEND_DIR,
                       args.backend_port, "/api/health", back_env)
        front_env = {**os.environ, "PRIVATE_API_URL": f"http://localhost:{args.backend_port}"} \
            if args.backend_port != 8000 else None
        front = Service("Frontend", "front",
                        [npm, "run", "dev", "--", "--port", str(args.frontend_port), "--strictPort"],
                        FRONTEND_DIR, args.frontend_port, "/", front_env)

        for s in (back, front):  # backend primero: el frontend lo necesita para cargar datos
            info(f"Iniciando {s.name}...")
            services.append(s)
            s.start()
            s.wait_ready(args.timeout)

        print()
        print(c("  Todo en marcha:", "ok"))
        print(f"    Sitio (frontend): {front.url}")
        print(f"    API (backend):    {back.url}   docs: {back.url}/docs")
        print(c("  Ctrl+C para detener ambos servidores.\n", "warn"))
        if not args.no_browser:
            webbrowser.open(front.url)
        return supervise(services)

    except StartupError as e:
        print()
        log("sistema", c(f"ERROR: {e}", "err"), "sys")
        return 1
    except KeyboardInterrupt:
        print()
        info("Interrumpido por el usuario.")
        return 0
    except Exception as e:  # error inesperado: mostrar y no dejar procesos huérfanos
        log("sistema", c(f"ERROR inesperado: {type(e).__name__}: {e}", "err"), "sys")
        return 1
    finally:
        if services:
            info("Deteniendo servidores...")
            for s in reversed(services):
                s.stop()
            ok("Servidores detenidos.")


if __name__ == "__main__":
    try:
        code = main()
    except KeyboardInterrupt:
        code = 0
    sys.exit(code)
