"""Genera internal/modules/cl/testdata/history_py.db: el backfill de Python sobre los fixtures de Chile.
Es el oráculo del test que compara la history.db escrita por Go con la de Python.
Uso (desde backend/): uv run python ../backend-go/scripts/gen_cl_history_fixture.py
"""

import shutil
import tempfile
from pathlib import Path

from fog_radar.modules.cl_stocks import history
from fog_radar.settings import Settings

root = Path(__file__).resolve().parents[2]
fixtures = root / "backend" / "tests" / "cl_stocks" / "fixtures"
dest = root / "backend-go" / "internal" / "modules" / "cl" / "testdata" / "history_py.db"
with tempfile.TemporaryDirectory() as tmp:
    data = Path(tmp)
    (data / "cl_stocks").mkdir()
    for f in fixtures.glob("*.csv"):
        shutil.copy(f, data / "cl_stocks" / f.name)
    s = Settings(data_dir=data, env="test")
    print(history.backfill(s))
    import sqlite3

    con = sqlite3.connect(data / "cl_stocks" / "history.db")
    con.execute("PRAGMA wal_checkpoint(TRUNCATE)")
    con.close()
    shutil.copy(data / "cl_stocks" / "history.db", dest)
print("->", dest)
