from __future__ import annotations

import pytest

from fog_radar.jobs import __main__ as cli


def test_list(capsys: pytest.CaptureFixture[str]) -> None:
    assert cli.main(["--list"]) == 0
    out = capsys.readouterr().out
    assert "cl_stocks" in out and "pendiente (Fase 3)" in out


def test_unported_module_returns_2(capsys: pytest.CaptureFixture[str]) -> None:
    assert cli.main(["etf"]) == 2
    assert "Fase 5" in capsys.readouterr().err
