"""CLI de pipelines.

Un módulo::

    uv run python -m fog_radar.jobs cl_stocks
    uv run python -m fog_radar.jobs etf --solo-analisis
    uv run python -m fog_radar.jobs tendencia --top-n 20 --max-weight 0.10

Varios en secuencia (nunca en paralelo, ver ``fog_radar.jobs.runner``)::

    uv run python -m fog_radar.jobs --todos                  # Chile → EEUU → Tendencia → ETF
    uv run python -m fog_radar.jobs --todos --solo etf,cl_stocks
    uv run python -m fog_radar.jobs --todos --offline        # sin descargar de Yahoo
    uv run python -m fog_radar.jobs --programa diario --trigger cron   # lo usa el crontab

Información::

    uv run python -m fog_radar.jobs --list                   # módulos y programación
    uv run python -m fog_radar.jobs --crontab                # crontab de producción
"""

from __future__ import annotations

import argparse
import sys
from datetime import datetime
from typing import Any

from fog_radar.core.logging import configure_logging
from fog_radar.jobs import PIPELINES
from fog_radar.jobs.schedule import OPCIONES_OFFLINE, ORDEN_TODOS, PROGRAMAS, PROGRAMAS_BY_KEY, crontab
from fog_radar.modules import MODULES, MODULES_BY_KEY
from fog_radar.settings import get_settings

EXIT_OK, EXIT_FALLA, EXIT_USO, EXIT_OCUPADO = 0, 1, 2, 3


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="fog_radar.jobs", description="Ejecuta pipelines batch (en secuencia)."
    )
    parser.add_argument("module", nargs="?", choices=sorted(MODULES_BY_KEY), help="Un solo módulo.")
    grupo = parser.add_mutually_exclusive_group()
    grupo.add_argument("--todos", action="store_true", help=f"Todos en secuencia: {' → '.join(ORDEN_TODOS)}.")
    grupo.add_argument("--programa", choices=sorted(PROGRAMAS_BY_KEY), help="Un programa del cron.")
    parser.add_argument(
        "--solo", help="Con --todos: módulos a correr, separados por coma (en el orden de --todos)."
    )
    parser.add_argument(
        "--offline",
        action="store_true",
        help="Sin descargar de Yahoo: re-procesa lo ya descargado (EEUU no tiene modo offline y se omite).",
    )
    parser.add_argument("--trigger", choices=["cli", "cron", "api"], default="cli")
    parser.add_argument(
        "--pausa", type=float, help="Segundos entre pipelines (por defecto PIPELINE_PAUSA_S)."
    )
    parser.add_argument(
        "--reintentos", type=int, help="Reintentos por pipeline (por defecto: 0 manual, 1 cron)."
    )
    parser.add_argument(
        "--esperar",
        type=float,
        metavar="SEG",
        help="Esperar hasta SEG segundos si otra corrida tiene el lock (cron: PIPELINE_LOCK_ESPERA_S).",
    )
    parser.add_argument("--list", action="store_true", help="Lista módulos y la programación.")
    parser.add_argument("--crontab", action="store_true", help="Imprime el crontab de producción.")
    parser.add_argument(
        "--backend-dir",
        default="/opt/fog-radar/backend",
        help="Con --crontab: carpeta backend/ en el servidor.",
    )
    seen: set[str] = set()
    for key, pipeline in PIPELINES.items():
        for opt in pipeline.options:
            if opt.flag in seen:
                continue
            if opt.con_valor:
                parser.add_argument(
                    opt.flag,
                    type=opt.convertir,
                    metavar="N" if opt.tipo is int else "X",
                    help=f"[{key}] {opt.help}",
                )
            else:
                parser.add_argument(opt.flag, action="store_true", help=f"[{key}] {opt.help}")
            seen.add(opt.flag)
    return parser


def _listar(settings) -> None:  # noqa: ANN001
    for m in MODULES:
        estado = "listo" if m.key in PIPELINES else f"pendiente (Fase {m.migration_phase})"
        print(f"{m.key:10s} {m.title:28s} {estado}")
    ahora = datetime.now(settings.tzinfo)
    print("\nProgramación (producción):")
    for p in PROGRAMAS:
        print(
            f"  {p.key:8s} {p.descripcion:32s} {' → '.join(p.modulos):36s} "
            f"próxima: {p.proxima(ahora):%Y-%m-%d %H:%M}"
        )


def main(argv: list[str] | None = None) -> int:
    parser = _parser()
    args = parser.parse_args(argv)

    settings = get_settings()
    configure_logging(settings.log_level)

    if args.crontab:
        print(crontab(args.backend_dir), end="")
        return EXIT_OK
    if args.list or (args.module is None and not args.todos and not args.programa):
        _listar(settings)
        return EXIT_OK

    # Qué módulos correr.
    if args.module:
        if args.todos or args.programa or args.solo:
            parser.error("Indica un módulo o --todos/--programa, no ambos.")
        modulos = [args.module]
    elif args.programa:
        modulos = list(PROGRAMAS_BY_KEY[args.programa].modulos)
    else:
        modulos = list(ORDEN_TODOS)
        if args.solo:
            pedidos = [m.strip() for m in args.solo.split(",") if m.strip()]
            desconocidos = [m for m in pedidos if m not in MODULES_BY_KEY]
            if desconocidos:
                print(f"Módulos desconocidos: {', '.join(desconocidos)}", file=sys.stderr)
                return EXIT_USO
            modulos = [m for m in ORDEN_TODOS if m in pedidos]

    # Banderas propias de cada módulo: solo con un módulo.
    todas = {o.dest: o for p in PIPELINES.values() for o in p.options}
    marcadas = {
        d: getattr(args, d)
        for d, o in todas.items()
        if (getattr(args, d, None) is not None if o.con_valor else getattr(args, d, False))
    }
    opciones: dict[str, dict[str, Any]] = {}
    if args.module:
        permitidas = {o.dest for o in PIPELINES[args.module].options}
        invalidas = set(marcadas) - permitidas
        if invalidas:
            print(
                f"Opciones no válidas para '{args.module}': {', '.join(sorted(invalidas))}", file=sys.stderr
            )
            return EXIT_USO
        opciones[args.module] = {todas[d].destino: v for d, v in marcadas.items()}
    elif marcadas:
        print(
            "Las banderas de módulo (p. ej. --skip-fetch) van con un solo módulo; usa --offline.",
            file=sys.stderr,
        )
        return EXIT_USO

    omitir: dict[str, str] = {}
    if args.offline:
        for m in modulos:
            if m in OPCIONES_OFFLINE:
                opciones[m] = {**opciones.get(m, {}), **OPCIONES_OFFLINE[m]}
            else:
                omitir[m] = "sin modo offline (siempre descarga de Yahoo)"

    from fog_radar.jobs.runner import OcupadoError, correr_secuencia, resumen

    cron = args.trigger == "cron"
    try:
        resultados = correr_secuencia(
            settings,
            modulos,
            trigger=args.trigger,
            opciones=opciones,
            omitir=omitir,
            pausa_s=args.pausa,
            reintentos=args.reintentos if args.reintentos is not None else (None if cron else 0),
            esperar_lock_s=args.esperar
            if args.esperar is not None
            else (settings.pipeline_lock_espera_s if cron else 0),
        )
    except OcupadoError as exc:
        print(str(exc), file=sys.stderr)
        return EXIT_OCUPADO

    if len(resultados) > 1 or args.todos or args.programa:
        print("\nResumen de la secuencia:\n" + resumen(resultados))
    return EXIT_FALLA if any(r.estado == "error" for r in resultados) else EXIT_OK


if __name__ == "__main__":
    raise SystemExit(main())
