# Servidor backend — corridas batch (cron)

Los pipelines corren en el servidor backend (donde viven los datos), **uno
detrás de otro**: Yahoo Finance bloquea las ráfagas de requests, así que nunca
hay dos descargando a la vez.

| Programa | Cuándo (America/Santiago) | Secuencia |
|---|---|---|
| `semanal` | lunes 06:00 | ETFs (~10 min) |
| `diario` | lunes a viernes 18:00 | Chile → EEUU → Tendencia (~10 min; 60 s de pausa entre módulos solo si el anterior falló o recibió 429) |

La programación vive en `backend-go/internal/core/schedule.go` (una sola fuente
de verdad): el archivo `crontab` de esta carpeta se genera desde ahí
(`fogradar job --crontab`) y la página `/estado` del sitio muestra la próxima
corrida de cada programa.

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
- El log detallado de todo el backend es `data/_jobs/logs/fogradar.log`.

## Instalación (Linux, sin Docker)

```bash
# 1. Zona horaria del servidor (cron de Debian/Ubuntu ignora CRON_TZ)
sudo timedatectl set-timezone America/Santiago

# 2. Binario y configuración en /opt/fog-radar/backend-go
cd /opt/fog-radar/backend-go
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o fogradar ./cmd/fogradar
cp .env.example .env   # API_KEY, FRED_API_KEY, ALERT_WEBHOOK_URL…

# 3. Carpeta de logs (del usuario que corre el backend)
sudo mkdir -p /var/log/fog-radar && sudo chown "$USER" /var/log/fog-radar

# 4. Instalar el crontab (reemplaza el crontab del usuario: revisar antes con crontab -l)
crontab /opt/fog-radar/deploy/backend/crontab
crontab -l
```

Si el código vive en otra carpeta, generar el crontab con:

```bash
./fogradar job --crontab --backend-dir /ruta/al/backend-go > mi-crontab
```

## Operación

```bash
./fogradar job --list                       # módulos y próximas corridas
./fogradar job --programa diario            # correr un programa ahora
./fogradar job --todos --esperar 3600       # todo, esperando si hay otra corrida
tail -f /var/log/fog-radar/pipelines-diario.log
```

Códigos de salida: `0` OK · `1` algún pipeline falló · `2` uso incorrecto · `3` otra corrida en curso.
