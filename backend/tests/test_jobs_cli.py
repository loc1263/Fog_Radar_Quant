from __future__ import annotations

import pytest

from fog_radar.jobs import PIPELINES
from fog_radar.jobs import __main__ as cli
from fog_radar.modules import MODULES


def test_list(capsys: pytest.CaptureFixture[str]) -> None:
    assert cli.main(["--list"]) == 0
    out = capsys.readouterr().out
    assert "pendiente" not in out  # Fase 6: los 4 pipelines están portados
    assert out.count("listo") == 4


def test_todos_los_modulos_tienen_pipeline() -> None:
    assert set(PIPELINES) == {m.key for m in MODULES}


def test_opcion_de_otro_modulo_rechazada(capsys: pytest.CaptureFixture[str]) -> None:
    assert cli.main(["cl_stocks", "--backfill"]) == 2
    assert "backfill" in capsys.readouterr().err
