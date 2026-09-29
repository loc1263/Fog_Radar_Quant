"""Paridad con ETF_Choice: mismo ``etf_perf_data`` + mismo histórico → mismos archivos.

Las salidas esperadas son las que dejó el proyecto original en ``backend/data`` para la
corrida del 24-09-2026; la rotación esperada se generó corriendo ``rotacion.py``
original sobre sus carpetas ``data`` y ``data_hist``.
"""

from __future__ import annotations

import json
from pathlib import Path

import pandas as pd
import pytest

from fog_radar.modules.etf import service, storage
from fog_radar.modules.etf.analysis import rotacion
from fog_radar.settings import Settings

from .conftest import FIXTURES, copiar_fixtures, expected, leer_bytes

SALIDAS = (
    "etf_top15_recomendados.csv",
    "etf_ranking_completo.csv",
    "etf_excluidos_outliers.csv",
    "etf_top15_simple.csv",
    "etf_momentum_ranking_completo.csv",
    "etf_momentum_top15_score_comp.csv",
    "etf_categorias_ranking.csv",
    "etf_russell3000_portfolio.csv",
    "etf_russell3000_resumen_buckets.csv",
    "historial_corridas.csv",
)


@pytest.fixture
def analizado(tmp_path: Path) -> Path:
    data = tmp_path / "current"
    hist = tmp_path / "hist"
    copiar_fixtures(FIXTURES / "input", data)
    copiar_fixtures(FIXTURES / "hist", hist)
    resultado = storage.correr_analisis(data, hist)
    assert resultado["avisos"] == []
    return data


@pytest.mark.parametrize("nombre", SALIDAS)
def test_salidas_byte_a_byte(analizado: Path, nombre: str) -> None:
    assert (analizado / nombre).read_bytes() == leer_bytes(expected(nombre)), nombre


def test_resumen_de_la_corrida(tmp_path: Path) -> None:
    data, hist = tmp_path / "current", tmp_path / "hist"
    copiar_fixtures(FIXTURES / "input", data)
    copiar_fixtures(FIXTURES / "hist", hist)
    r = storage.correr_analisis(data, hist)
    assert (r["fecha_tag"], r["n_universo_bruto"], r["n_universo_depurado"], r["n_outliers"]) == (
        "20260924",
        1189,
        683,
        8,
    )
    assert r["top15"]["Ticker"].iloc[0] == "CRAK"


def _limpia(registros: list[dict]) -> list[dict]:
    return [{k: (None if isinstance(v, float) and v != v else v) for k, v in r.items()} for r in registros]


def test_rotacion_igual_al_original(settings: Settings, etf_corrida: Path) -> None:
    esperado = json.loads((FIXTURES / "expected" / "rotacion.json").read_text(encoding="utf-8"))
    top15 = pd.read_csv(etf_corrida / storage.TOP15)
    ranking = pd.read_csv(etf_corrida / storage.RANKING)
    excl = pd.read_csv(etf_corrida / storage.EXCLUIDOS)
    res = rotacion.generar_datos_rotacion(
        str(etf_corrida), str(storage.hist_dir(settings)), top15, excl, ranking
    )
    assert res is not None
    for clave in ("mantienen", "nuevos", "salidas"):
        obtenido = json.loads(res[clave].to_json(orient="records"))
        assert _limpia(obtenido) == _limpia(esperado[clave]), clave
    diversificacion = json.loads(rotacion.diversificacion_top15(top15).to_json(orient="records"))
    assert diversificacion == esperado["diversificacion"]


def test_api_rotacion_mapea_las_columnas(settings: Settings, etf_corrida: Path) -> None:
    esperado = json.loads((FIXTURES / "expected" / "rotacion.json").read_text(encoding="utf-8"))
    rot = service.rotacion_vs_anterior(settings)
    assert rot.disponible
    assert [f.ticker for f in rot.mantienen] == [r["Ticker"] for r in esperado["mantienen"]]
    assert [f.racha for f in rot.mantienen] == [r["Racha"] for r in esperado["mantienen"]]
    assert [f.destino for f in rot.salidas] == [r["Destino"] for r in esperado["salidas"]]
    assert [f.procedencia for f in rot.nuevos] == [r["Procedencia"] for r in esperado["nuevos"]]
    assert str(rot.fecha_previa) == "2026-09-22"
