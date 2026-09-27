"""Paridad con Stock_Choice_Nacional: mismo CSV de indicadores → mismo ranking.

Los ``ranking_acciones_*.csv`` de ``fixtures/`` fueron generados por el
``scoring_acciones.py`` original a partir del CSV de indicadores con el mismo
timestamp (copiados del respaldo ``_backups/Stock_Choice_Nacional_2026-09-27.zip``).
"""

from __future__ import annotations

from pathlib import Path

import pandas as pd
import pytest

from fog_radar.modules.cl_stocks.scoring import construir_ranking

FIXTURES = Path(__file__).with_name("fixtures")
CORRIDAS = ["20260924_221437", "20260925_183129"]


@pytest.mark.parametrize("ts", CORRIDAS)
def test_ranking_identico_al_original(ts: str) -> None:
    indicadores = pd.read_csv(FIXTURES / f"indicadores_acciones_{ts}.csv")
    esperado = pd.read_csv(FIXTURES / f"ranking_acciones_{ts}.csv")
    obtenido = construir_ranking(indicadores)

    assert list(obtenido.columns) == list(esperado.columns)
    # Fila a fila: mismo orden de símbolos y mismos valores (a 4 decimales, como el CSV).
    assert obtenido["Símbolo"].tolist() == esperado["Símbolo"].tolist()
    obtenido["Grupo Financiero Duplicado"] = obtenido["Grupo Financiero Duplicado"].astype(float)
    pd.testing.assert_frame_equal(obtenido, esperado, check_dtype=False, atol=1e-9, rtol=0)


def test_csv_escrito_se_relee_igual(tmp_path: Path) -> None:
    ts = CORRIDAS[-1]
    obtenido = construir_ranking(pd.read_csv(FIXTURES / f"indicadores_acciones_{ts}.csv"))
    obtenido.to_csv(tmp_path / "r.csv", index=False)
    esperado = (FIXTURES / f"ranking_acciones_{ts}.csv").read_text(encoding="utf-8")
    assert (tmp_path / "r.csv").read_text(encoding="utf-8") == esperado
