#!/usr/bin/env python3
"""Corre a pedido TODOS los pipelines, uno detrás de otro (desarrollo, Windows o Linux).

Uso (desde la raíz del repo, o doble clic en correr_pipelines.bat):

    python scripts/run_all_pipelines.py                    # Chile → EEUU → Tendencia → ETF
    python scripts/run_all_pipelines.py --solo etf,tendencia
    python scripts/run_all_pipelines.py --offline          # sin Yahoo: re-procesa lo descargado
    python scripts/run_all_pipelines.py --pausa 120 --si   # sin confirmación

Nunca corre dos pipelines a la vez: usa la misma secuencia que el cron de
producción (``python -m fog_radar.jobs --todos``), con pausa entre módulos y un
lock global. Si el cron u otra consola ya está corriendo pipelines, este script
termina avisando (o espera, con ``--esperar``). La API puede quedar levantada
(iniciar_dev.py): los datos se escriben de forma atómica.

La salida se ve en la consola y queda en ``data/_jobs/logs/manual_<fecha>.log``.
Cada módulo queda además en la página /estado del sitio.
"""

from __future__ import annotations

import argparse
import os
import shutil
import subprocess
import sys
from datetime import datetime
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BACKEND = ROOT / "backend"
LOGS = ROOT / "data" / "_jobs" / "logs"
ORDEN = ("cl_stocks", "us_stocks", "tendencia", "etf")
DURACION_MIN = {"cl_stocks": 10, "us_stocks": 12, "tendencia": 10, "etf": 30}
NOMBRES = {"cl_stocks": "Chile (IPSA)", "us_stocks": "EEUU", "tendencia": "Tendencia", "etf": "ETFs"}


def _parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(description="Corre todos los pipelines en secuencia (a pedido).")
    p.add_argument("--solo", help=f"Módulos separados por coma ({', '.join(ORDEN)}).")
    p.add_argument("--offline", action="store_true", help="Sin descargar de Yahoo (EEUU se omite).")
    p.add_argument("--pausa", type=float, help="Segundos entre pipelines (por defecto 60).")
    p.add_argument(
        "--reintentos", type=int, default=0, help="Reintentos por pipeline que falla (por defecto 0)."
    )
    p.add_argument(
        "--esperar", type=float, metavar="SEG", help="Esperar SEG segundos si otra corrida está en curso."
    )
    p.add_argument("--si", action="store_true", help="No pedir confirmación.")
    return p


def _comando_python() -> list[str]:
    """``uv run`` si está instalado; si no, el Python del entorno virtual del backend."""
    if shutil.which("uv"):
        return ["uv", "run", "python"]
    venv = BACKEND / ".venv" / ("Scripts/python.exe" if os.name == "nt" else "bin/python")
    if venv.is_file():
        return [str(venv)]
    sys.exit("No se encontró 'uv' ni backend/.venv. Instalar uv: https://docs.astral.sh/uv/")


def main() -> int:
    args = _parser().parse_args()
    modulos = list(ORDEN)
    if args.solo:
        pedidos = {m.strip() for m in args.solo.split(",") if m.strip()}
        desconocidos = pedidos - set(ORDEN)
        if desconocidos:
            print(f"Módulos desconocidos: {', '.join(sorted(desconocidos))}. Válidos: {', '.join(ORDEN)}")
            return 2
        modulos = [m for m in ORDEN if m in pedidos]
    if args.offline and "us_stocks" in modulos:
        print("Nota: EEUU no tiene modo offline (siempre descarga de Yahoo); con --offline se omite.")

    minutos = 2 if args.offline else sum(DURACION_MIN[m] for m in modulos)
    print("Fog Radar Quant — corrida manual de pipelines (en secuencia, nunca en paralelo)")
    print(f"  Orden: {' → '.join(NOMBRES[m] for m in modulos)}")
    print(f"  Modo: {'offline (sin Yahoo)' if args.offline else 'completo (descarga de Yahoo)'}")
    print(f"  Duración estimada: ~{minutos} min")
    if not args.si:
        try:
            if input("¿Continuar? [s/N] ").strip().lower() not in ("s", "si", "sí", "y", "yes"):
                print("Cancelado.")
                return 0
        except EOFError:
            return 0

    cmd = [*_comando_python(), "-m", "fog_radar.jobs", "--todos", "--trigger", "cli"]
    if args.solo:
        cmd += ["--solo", ",".join(modulos)]
    if args.offline:
        cmd.append("--offline")
    if args.pausa is not None:
        cmd += ["--pausa", str(args.pausa)]
    cmd += ["--reintentos", str(args.reintentos)]
    if args.esperar is not None:
        cmd += ["--esperar", str(args.esperar)]

    LOGS.mkdir(parents=True, exist_ok=True)
    log_path = LOGS / f"manual_{datetime.now():%Y%m%d_%H%M%S}.log"
    env = {**os.environ, "PYTHONUTF8": "1", "PYTHONIOENCODING": "utf-8"}
    print(f"  Log: {log_path.relative_to(ROOT)}\n")

    inicio = datetime.now()
    with (
        log_path.open("w", encoding="utf-8") as log,
        subprocess.Popen(
            cmd,
            cwd=BACKEND,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            encoding="utf-8",
            errors="replace",
        ) as proc,
    ):
        assert proc.stdout is not None
        try:
            for linea in proc.stdout:
                sys.stdout.write(linea)
                sys.stdout.flush()
                log.write(linea)
            codigo = proc.wait()
        except KeyboardInterrupt:
            proc.terminate()
            print("\nInterrumpido por el usuario. El módulo en curso queda como error en /estado.")
            return 130

    duracion = (datetime.now() - inicio).total_seconds() / 60
    mensajes = {
        0: "Todos los pipelines terminaron OK.",
        1: "Terminó con errores en algún pipeline (ver el resumen de arriba y /estado).",
        3: "Ya hay otra corrida de pipelines en curso (cron u otra consola). Reintenta más tarde o usa --esperar.",
    }
    print(f"\n{mensajes.get(codigo, f'Terminó con código {codigo}.')} ({duracion:.1f} min)")
    return codigo


if __name__ == "__main__":
    raise SystemExit(main())
