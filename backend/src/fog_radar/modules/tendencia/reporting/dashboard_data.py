"""
Exporta los resultados de la corrida actual como JSON para el frontend
(Vue + Tailwind, en frontend/, hermano de este backend/), y además guarda
la misma corrida en la base histórica SQLite (ver storage.history_writer)
para poder analizar tendencias entre corridas (vista "Historial" del
frontend, servida por api.app).

A diferencia del informe PDF (un snapshot fechado por corrida), el JSON
se escribe SIEMPRE en la misma ruta fija dentro de frontend/public/ (ver
FRONTEND_PUBLIC_DATA_DIR más abajo), sobrescribiendo la corrida anterior.
Así el frontend puede mostrar siempre "la última corrida" con un simple
`fetch('/data/latest/dashboard_data.json')` — sin necesidad de un
backend/API corriendo, sin conocer el nombre de la carpeta de la corrida
(output/<YYYYMMDD>_<NN>/), y sin lógica de "encontrar la más reciente"
del lado del navegador. El guardado en la base histórica es aparte —
ver storage.history_writer.store_run() — y sí requiere que corra
api_server.py para que el frontend pueda consultarlo (vista Historial).

La lectura de los archivos de la corrida vive en reporting.run_data
(compartida con el informe PDF): este módulo solo arma el JSON.

Ejecutar (después de markowitz, trend_forecast y momentum, vía
wrapper de compatibilidad en la raíz del proyecto):
    python step07_export_dashboard_data.py

Genera:
    output/<YYYYMMDD>_<NN>/dashboard_data.json   (archivado con esta corrida)
    ../frontend/public/data/latest/dashboard_data.json (el frontend SIEMPRE lee esta ruta)
    data/history.db  (fila(s) nueva(s) en runs/analyses/holdings/sectors/momentum)
"""

import json
import os
from datetime import datetime

from fog_radar.modules.tendencia import config
from fog_radar.modules.tendencia.reporting.run_data import compute_sector_summary, load_analysis
from fog_radar.modules.tendencia.storage import history_writer
from fog_radar.modules.tendencia.storage.db import history_db_path

# Fog Radar: ya no se copia a frontend/public/ — el sitio lee la última corrida
# por la API (GET /api/tendencia/latest), que sirve el JSON archivado abajo.


def _records(df, index_as=None):
    """
    DataFrame -> list[dict] serializable a JSON. Usa el propio
    `DataFrame.to_json` (vía un round-trip json.loads) en vez de
    `to_dict("records")` porque convierte NaN -> null y tipos numpy ->
    nativos de Python automáticamente; a mano hay que replicar ambas
    cosas o `json.dump` falla o produce JSON inválido.

    `index_as`: si se da, resetea el índice y lo deja como columna con
    ese nombre — para DataFrames donde el ticker/sector vive en el
    índice (p. ej. `merged`, indexado por ticker).
    """
    if index_as:
        df = df.reset_index()
        if "index" in df.columns:
            df = df.rename(columns={"index": index_as})
    return json.loads(df.to_json(orient="records"))


def build_analysis_payload(spec, data):
    """Payload JSON de un análisis (ver settings.AnalysisSpec y run_data.AnalysisData)."""
    merged = data.merged
    return {
        "key": spec.key,
        "label": spec.label,
        "universe_size": len(spec.tickers),
        "n_selected": len(data.perf.get("tickers_seleccionados", merged.index)),
        "performance": data.perf,
        "holdings": _records(merged, index_as="ticker"),
        "sectors": _records(compute_sector_summary(merged), index_as="sector"),
        "momentum": _records(data.momentum),
        "momentum_semanal": _records(data.momentum_weekly),
        "portfolio_options": [
            {**opt, "holdings": _records(opt["holdings"], index_as="ticker")}
            for opt in data.reduced_portfolios
        ],
    }


def build_dashboard_payload(run_dir):
    """
    Arma el payload completo (todos los análisis disponibles esa corrida,
    ver config.ANALYSES) a partir de los archivos que modeling.markowitz/
    trend_forecast/momentum ya escribieron en `run_dir`. Los
    análisis complementarios que se hayan omitido (universo insuficiente)
    simplemente no aparecen en `analyses`.
    """
    analyses_payload = []
    notes = []
    for spec in config.ANALYSES:
        data = load_analysis(run_dir, suffix=spec.key)
        if data is None:
            continue
        if spec.key == "":
            # Contexto de mercado: es macro, no depende del universo, se
            # toma del análisis principal (siempre presente).
            notes = data.notes
        analyses_payload.append(build_analysis_payload(spec, data))

    if not analyses_payload:
        raise RuntimeError("No se generó ningún análisis (ver modeling.markowitz); nada que exportar.")

    return {
        "generated_at": datetime.now().isoformat(),
        "run_dir": os.path.basename(run_dir),
        "universe_source": config.UNIVERSE_SOURCE,
        "market_regime_notes": notes,
        "analyses": analyses_payload,
    }


def main():
    run_dir = config.get_run_dir()
    payload = build_dashboard_payload(run_dir)

    archived_path = os.path.join(run_dir, "dashboard_data.json")
    # Escritura atómica: la API nunca lee un JSON a medias.
    tmp_path = archived_path + ".tmp"
    with open(tmp_path, "w", encoding="utf-8") as f:
        json.dump(payload, f, indent=2, ensure_ascii=False)
    os.replace(tmp_path, archived_path)

    print(f"Datos del dashboard exportados: {archived_path}")

    try:
        history_writer.store_run(payload)
        print(f"Corrida guardada en la base histórica: {history_db_path()}")
    except Exception as e:
        # No es fatal para esta corrida (el JSON/CSV de siempre ya se
        # escribieron arriba) — pero se avisa fuerte, porque significa
        # que esta corrida no va a aparecer en la vista "Historial" hasta
        # que se corra backfill_history.py o se resuelva el problema.
        print(
            f"AVISO: no se pudo guardar la corrida en la base histórica ({e}). "
            "El JSON/CSV de esta corrida sí se generaron bien; podés reintentar "
            "el guardado en la base corriendo: python -m fog_radar.jobs tendencia --backfill"
        )

    return payload


if __name__ == "__main__":
    main()
