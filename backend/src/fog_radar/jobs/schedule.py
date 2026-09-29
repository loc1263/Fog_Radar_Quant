"""Programación de las corridas batch (una sola fuente de verdad).

Producción (Linux): el crontab del servidor backend se genera desde acá
(``python -m fog_radar.jobs --crontab``) y la página ``/estado`` muestra la
próxima corrida de cada programa con estos mismos datos.

- **semanal** — ETFs, los lunes a las 06:00.
- **diario** — Chile, EEUU y Tendencia, de lunes a viernes a las 18:00, en ese
  orden y uno detrás de otro (nunca en paralelo: Yahoo Finance bloquea por
  ráfagas de requests).

Las horas son de ``Settings.timezone`` (America/Santiago).
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta

DIAS = ("lunes", "martes", "miércoles", "jueves", "viernes", "sábado", "domingo")


@dataclass(frozen=True)
class Programa:
    key: str
    titulo: str
    modulos: tuple[str, ...]
    """Módulos en el orden en que se corren (secuencia)."""
    dias: tuple[int, ...]
    """Días de la semana (0 = lunes … 6 = domingo)."""
    hora: int
    minuto: int = 0

    @property
    def cron(self) -> str:
        """Expresión cron (en cron el lunes es 1 y el domingo 0/7)."""
        dias = sorted(d + 1 for d in self.dias)
        if len(dias) > 1 and dias == list(range(dias[0], dias[-1] + 1)):
            campo = f"{dias[0]}-{dias[-1]}"
        else:
            campo = ",".join(str(d) for d in dias)
        return f"{self.minuto} {self.hora} * * {campo}"

    @property
    def descripcion(self) -> str:
        if self.dias == (0, 1, 2, 3, 4):
            cuando = "de lunes a viernes"
        elif len(self.dias) == 1:
            cuando = f"los {DIAS[self.dias[0]]}"
        else:
            cuando = ", ".join(DIAS[d] for d in self.dias)
        return f"{cuando} a las {self.hora:02d}:{self.minuto:02d}"

    def proxima(self, ahora: datetime) -> datetime:
        """Próxima ejecución estrictamente posterior a ``ahora`` (misma zona horaria)."""
        base = ahora.replace(hour=self.hora, minute=self.minuto, second=0, microsecond=0)
        for delta in range(8):
            candidata = base + timedelta(days=delta)
            if candidata.weekday() in self.dias and candidata > ahora:
                return candidata
        raise ValueError(f"El programa {self.key} no tiene días válidos")

    def anterior(self, ahora: datetime) -> datetime:
        """Última ejecución programada en o antes de ``ahora`` (misma zona horaria)."""
        base = ahora.replace(hour=self.hora, minute=self.minuto, second=0, microsecond=0)
        for delta in range(8):
            candidata = base - timedelta(days=delta)
            if candidata.weekday() in self.dias and candidata <= ahora:
                return candidata
        raise ValueError(f"El programa {self.key} no tiene días válidos")


PROGRAMAS: tuple[Programa, ...] = (
    Programa("semanal", "ETFs", ("etf",), dias=(0,), hora=6),
    Programa(
        "diario",
        "Chile, EEUU y Tendencia",
        ("cl_stocks", "us_stocks", "tendencia"),
        dias=(0, 1, 2, 3, 4),
        hora=18,
    ),
)
PROGRAMAS_BY_KEY = {p.key: p for p in PROGRAMAS}
PROGRAMA_DE_MODULO: dict[str, Programa] = {m: p for p in PROGRAMAS for m in p.modulos}

# Corrida manual de "todo" (desarrollo): los diarios primero y ETF (el más largo, ~30 min) al final.
ORDEN_TODOS: tuple[str, ...] = ("cl_stocks", "us_stocks", "tendencia", "etf")

# Opciones que corren un módulo sin descargar de Yahoo (``--offline``), sobre los datos ya descargados.
OPCIONES_OFFLINE: dict[str, dict[str, bool]] = {
    "cl_stocks": {"solo_scoring": True},
    "etf": {"solo_analisis": True},
    "tendencia": {"skip_fetch": True, "skip_discovery": True},
    # us_stocks no tiene modo sin descarga: con --offline se omite.
}


def crontab(backend_dir: str, uv: str = "/usr/local/bin/uv", log_dir: str = "/var/log/fog-radar") -> str:
    """Contenido del crontab del servidor backend."""
    lineas = [
        "# Fog Radar Quant — corridas batch (generado con: python -m fog_radar.jobs --crontab)",
        "# Horas de America/Santiago. CRON_TZ lo respeta cronie; con cron de Debian/Ubuntu",
        "# la zona del servidor debe ser America/Santiago (timedatectl set-timezone America/Santiago).",
        "CRON_TZ=America/Santiago",
        "SHELL=/bin/bash",
        "PATH=/usr/local/bin:/usr/bin:/bin",
        "",
    ]
    for p in PROGRAMAS:
        lineas.append(f"# {p.titulo}: {p.descripcion} ({' → '.join(p.modulos)})")
        lineas.append(
            f"{p.cron} cd {backend_dir} && {uv} run --frozen python -m fog_radar.jobs "
            f"--programa {p.key} --trigger cron >> {log_dir}/pipelines-{p.key}.log 2>&1"
        )
    return "\n".join(lineas) + "\n"
