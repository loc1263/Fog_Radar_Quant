"""Paridad con Stock_Tendencia: mismos datos procesados + mismo universo y macro →
mismas salidas que la corrida real 20260926_01 del original.

Se corren los pasos 4-7 (Markowitz, tendencia, momentum y el payload del
dashboard) sobre ``data/processed`` de esa corrida. La comparación es numérica
(rtol 1e-7): Windows y Linux difieren en el último dígito de algunos floats
(p. ej. 0.40082043126284495 vs 0.40082043126284517). Los pesos (redondeados a 5 decimales) y los textos son idénticos.

Excepción documentada (docs/PARIDAD.md, Fase 7): la proyección a 6 meses se
corrigió: el original usaba el CAGR μ como si fuera una tasa continua y además le
restaba σ²/2 (exp((μ − σ²/2)·T)); la mediana correcta del GBM con un CAGR es
P·(1+μ)^T. Esas columnas se verifican aparte:
nuevo = original · exp((ln(1+μ) − μ + σ²/2)·T).
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

# Salidas de la proyección GBM corregida en la Fase 7 (ver docstring).
PROYECCION = ("precio_proyectado_6m_usd", "precio_min_90pct_6m_usd", "precio_max_90pct_6m_usd")
CORREGIDAS = {*PROYECCION, "retorno_esperado_6m_pct", "retorno_6m_prom"}
T_6M = 26 / 52


def _sin_corregidas(x: Any) -> Any:
    if isinstance(x, dict):
        return {k: _sin_corregidas(v) for k, v in x.items() if k not in CORREGIDAS}
    if isinstance(x, list):
        return [_sin_corregidas(v) for v in x]
    return x


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
    if nombre.startswith("trend_forecast"):
        _proyeccion_corregida(pd.read_csv(obtenido), pd.read_csv(esperado))
    elif nombre.endswith(".csv"):
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
    assert _cerca(_sin_corregidas(obtenido), _sin_corregidas(esperado))


def _proyeccion_corregida(obt: pd.DataFrame, esp: pd.DataFrame) -> None:
    """Todo igual al original salvo la proyección, que cambia en exp((ln(1+μ) − μ + σ²/2)·T)."""
    obt = obt.sort_values("ticker").reset_index(drop=True)
    esp = esp.sort_values("ticker").reset_index(drop=True)
    iguales = [c for c in esp.columns if c not in CORREGIDAS]
    pd.testing.assert_frame_equal(obt[iguales], esp[iguales], rtol=1e-7)
    sigma = esp["volatilidad_anual_pct"] / 100
    mu = esp["retorno_esperado_anual_pct"] / 100
    factor = ((mu.map(math.log1p) - mu + sigma**2 / 2) * T_6M).map(math.exp)
    for c in PROYECCION:
        # Los CSV vienen redondeados a centavos: tolerancia de redondeo.
        assert ((obt[c] - esp[c] * factor).abs() <= 0.01 + 1e-4 * obt[c]).all(), c
    # Mediana = P·(1+CAGR)^T.
    mediana = esp["precio_actual_usd"] * (1 + esp["retorno_esperado_anual_pct"] / 100) ** T_6M
    assert ((obt["precio_proyectado_6m_usd"] - mediana).abs() <= 0.02 + 2e-4 * mediana).all()
