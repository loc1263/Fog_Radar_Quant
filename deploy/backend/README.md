# Servidor backend — corridas batch (cron)

Los pipelines corren en el servidor backend (donde viven los datos), **uno
detrás de otro**: Yahoo Finance bloquea las ráfagas de requests, así que nunca
hay dos descargando a la vez.

| Programa | Cuándo (America/Santiago) | Secuencia |
|---|---|---|
| `semanal` | lunes 06:00 | ETFs (~30 min) |
| `diario` | lunes a viernes 18:00 | Chile → EEUU → Tendencia (~35 min, 60 s de pausa entre cada uno) |

La programación vive en `backend/src/fog_radar/jobs/schedule.py` (una sola fuente
de verdad): el archivo `crontab` de esta carpeta se genera desde ahí y la página
`/estado` del sitio muestra la próxima corrida de cada programa. Un test
(`tests/test_jobs_runner.py`) falla si el `crontab` versionado no coincide.

## Garantías

- **Lock global entre procesos** (`data/_jobs/.pipelines.lock`): si el cron del
  lunes 18:00 encuentra al ETF todavía corriendo, espera (hasta 6 h,
  `PIPELINE_LOCK_ESPERA_S`) en vez de correr en paralelo. Una corrida manual
  mientras corre el cron termina con código 3 ("ocupado").
- **Reintento**: en las corridas del cron, un pipeline que falla se reintenta
  una vez tras 10 min (`PIPELINE_REINTENTOS`, `PIPELINE_ESPERA_REINTENTO_S`) y la
  secuencia sigue con el módulo siguiente.
- **Aviso**: si algo falla y `ALERT_WEBHOOK_URL` está definida (Slack, Discord,
  Teams), llega un mensaje con el módulo y el error.
- Cada intento queda en `data/_jobs/<modulo>.jsonl` → `/api/jobs` y `/estado`.

## Instalación (Linux, sin Docker)

```bash
# 1. Zona horaria del servidor (cron de Debian/Ubuntu ignora CRON_TZ)
sudo timedatectl set-timezone America/Santiago

# 2. Código en /opt/fog-radar y dependencias
cd /opt/fog-radar/backend && uv sync --frozen
cp .env.example .env   # API_KEY, FRED_API_KEY, ALERT_WEBHOOK_URL…

# 3. Carpeta de logs (del usuario que corre el backend)
sudo mkdir -p /var/log/fog-radar && sudo chown "$USER" /var/log/fog-radar

# 4. Instalar el crontab (reemplaza el crontab del usuario: revisar antes con crontab -l)
crontab /opt/fog-radar/deploy/backend/crontab
crontab -l
```

Si el código vive en otra carpeta o `uv` en otra ruta, generar el crontab con:

```bash
uv run python -m fog_radar.jobs --crontab --backend-dir /ruta/al/backend > mi-crontab
```

## Operación

```bash
uv run python -m fog_radar.jobs --list                       # módulos y próximas corridas
uv run python -m fog_radar.jobs --programa diario            # correr un programa ahora
uv run python -m fog_radar.jobs --todos --esperar 3600       # todo, esperando si hay otra corrida
tail -f /var/log/fog-radar/pipelines-diario.log
```

Códigos de salida: `0` OK · `1` algún pipeline falló · `2` uso incorrecto · `3` otra corrida en curso.

La imagen Docker y el cron dentro del contenedor quedan para la Fase 8.
