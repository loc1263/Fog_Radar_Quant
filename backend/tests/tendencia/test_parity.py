"""Paridad con Stock_Tendencia: mismos datos procesados + mismo universo y macro →
mismas salidas que la corrida real 20260926_01 del original.

Se corren los pasos 4-7 (Markowitz, tendencia, momentum y el payload del
dashboard) sobre ``data/processed`` de esa corrida. La comparación es numérica
(rtol 1e-7): Windows y Linux difieren en el último dígito de algunos floats
(p. ej. 0.40082043126284495 vs 0.40082043126284517). Los pesos (redondeados a 5 decimales) y los textos son idénticos.
"""

from __future__ import annotations

import importlib
import json
import math
from collections.abc import Iterator
from pathlib import Path
from typing import Any

import pandas as pd
import pytest

from fog_radar.modules.tendencia import paths

from .helpers import FIXTURES, RUN, copiar_arbol

ESPERADO = FIXTURES / "expected" / RUN


def _cerca(a: Any, b: Any) -> bool:
    if isinstance(a, dict):
        return isinstance(b, dict) and a.keys() == b.keys() and all(_cerca(a[k], b[k]) for k in a)
    if isinstance(a, list):
        return (
            isinstance(b, list) and len(a) == len(b) and all(_cerca(x, y) for x, y in zip(a, b, strict=True))
        )
    if isinstance(a, float) or isinstance(b, float):
        if a is None or b is None:
            return a is b
        return math.isclose(a, b, rel_tol=1e-7, abs_tol=1e-9)
    return a == b


@pytest.fixture(scope="module")
def corrida(tmp_path_factory: pytest.TempPathFactory, _tendencia_root: Path) -> Iterator[tuple[Path, dict]]:
    root = tmp_path_factory.mktemp("paridad")
    copiar_arbol(FIXTURES / "data", root)
    from fog_radar.modules.tendencia import config

    paths.set_root(root)
    importlib.reload(config)
    try:
        from fog_radar.modules.tendencia.modeling import markowitz, momentum, trend_forecast
        from fog_radar.modules.tendencia.reporting import dashboard_data

        run_dir = Path(config.get_run_dir(new=True))
        markowitz.main()
        trend_forecast.main()
        momentum.main()
        payload = dashboard_data.build_dashboard_payload(str(run_dir))
        yield run_dir, payload
    finally:
        paths.set_root(_tendencia_root)
        importlib.reload(config)


SALIDAS = sorted(
    p.name
    for p in ESPERADO.iterdir()
    if p.suffix in (".csv", ".json", ".txt") and p.name != "run_manifest.json"
)


@pytest.mark.parametrize("nombre", SALIDAS)
def test_salida_igual_al_original(corrida: tuple[Path, dict], nombre: str) -> None:
    run_dir, _ = corrida
    obtenido, esperado = run_dir / nombre, ESPERADO / nombre
    if nombre.endswith(".csv"):
        pd.testing.assert_frame_equal(pd.read_csv(obtenido), pd.read_csv(esperado), rtol=1e-7)
    elif nombre.endswith(".json"):
        assert _cerca(
            json.loads(obtenido.read_text(encoding="utf-8")), json.loads(esperado.read_text(encoding="utf-8"))
        )
    else:
        assert (
            obtenido.read_text(encoding="utf-8").splitlines()
            == esperado.read_text(encoding="utf-8").splitlines()
        )


def test_payload_del_dashboard_igual_al_original(corrida: tuple[Path, dict]) -> None:
    import gzip

    _, payload = corrida
    with gzip.open(ESPERADO / "dashboard_data.json.gz", "rt", encoding="utf-8") as fh:
        esperado = json.load(fh)
    obtenido = json.loads(json.dumps(payload))
    for d in (obtenido, esperado):
        d.pop("generated_at")
        d.pop("run_dir")
    assert _cerca(obtenido, esperado)
