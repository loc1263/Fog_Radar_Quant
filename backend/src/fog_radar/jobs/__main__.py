"""CLI de pipelines.

Ejemplos::

    uv run python -m fog_radar.jobs --list
    uv run python -m fog_radar.jobs cl_stocks --trigger cron
    uv run python -m fog_radar.jobs cl_stocks --solo-scoring
"""

from __future__ import annotations

import argparse
import sys

from fog_radar.core.jobs import JobRegistry
from fog_radar.core.logging import configure_logging
from fog_radar.jobs import PIPELINES
from fog_radar.modules import MODULES, MODULES_BY_KEY
from fog_radar.settings import get_settings


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="fog_radar.jobs", description="Ejecuta el pipeline de un módulo.")
    parser.add_argument("module", nargs="?", choices=sorted(MODULES_BY_KEY))
    parser.add_argument("--trigger", choices=["cli", "cron", "api"], default="cli")
    parser.add_argument("--list", action="store_true", help="Lista módulos y si su pipeline ya está portado.")
    seen: set[str] = set()
    for key, pipeline in PIPELINES.items():
        for opt in pipeline.options:
            if opt.flag not in seen:
                parser.add_argument(opt.flag, action="store_true", help=f"[{key}] {opt.help}")
                seen.add(opt.flag)
    return parser


def main(argv: list[str] | None = None) -> int:
    args = _parser().parse_args(argv)

    settings = get_settings()
    configure_logging(settings.log_level)

    if args.list or args.module is None:
        for m in MODULES:
            estado = "listo" if m.key in PIPELINES else f"pendiente (Fase {m.migration_phase})"
            print(f"{m.key:10s} {m.title:28s} {estado}")
        return 0

    pipeline = PIPELINES.get(args.module)
    if pipeline is None:
        info = MODULES_BY_KEY[args.module]
        print(
            f"El pipeline de '{args.module}' aún no está portado (Fase {info.migration_phase}).",
            file=sys.stderr,
        )
        return 2

    # Opciones de otros módulos no aplican a este.
    allowed = {o.dest for o in pipeline.options}
    all_dests = {o.dest for p in PIPELINES.values() for o in p.options}
    invalid = [d for d in all_dests - allowed if getattr(args, d, False)]
    if invalid:
        print(f"Opciones no válidas para '{args.module}': {', '.join(invalid)}", file=sys.stderr)
        return 2
    opts = {d: True for d in allowed if getattr(args, d, False)}

    registry = JobRegistry(settings.jobs_dir)
    if registry.is_running(args.module):
        print(f"Ya hay una corrida de '{args.module}' en curso.", file=sys.stderr)
        return 3
    with registry.track(args.module, trigger=args.trigger) as job:
        job.message = pipeline.run(settings, **opts)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
