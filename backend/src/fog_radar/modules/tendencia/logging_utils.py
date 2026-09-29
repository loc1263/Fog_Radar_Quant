"""
Utilidades de logging compartidas por todos los pasos del pipeline.

Antes de esta refactorización, cada step*.py llamaba a
``logging.basicConfig(handlers=[...])`` de forma independiente. Como
``run_pipeline.py`` importa todos los pasos dentro del MISMO proceso de
Python, y ``logging.basicConfig`` solo tiene efecto la PRIMERA vez que se
llama (si el root logger ya tiene handlers, las llamadas siguientes son
no-op), en la práctica solo los mensajes del primer paso llegaban a su
archivo .log correspondiente cuando se corría el pipeline completo con
``run_pipeline.py``; el resto de los pasos escribía silenciosamente en el
handler del primero. Ver mejoras_pendientes.txt (ID LOG-1).

``get_logger`` evita el problema: crea (o reutiliza) un logger CON NOMBRE
propio y le agrega sus propios handlers directamente, en vez de depender
del root logger compartido. Es idempotente: llamarlo más de una vez con el
mismo `name` no duplica handlers (importante porque `importlib.reload`
puede volver a ejecutar el módulo que lo llama).
"""

import logging
import os


def get_logger(name, base_dir, log_filename=None, level=logging.INFO):
    """Devuelve un logger configurado con handlers propios (consola + archivo)."""
    logger = logging.getLogger(name)
    logger.setLevel(level)
    logger.propagate = False  # no duplicar hacia el root logger

    if logger.handlers:
        return logger  # ya configurado (p.ej. tras un importlib.reload)

    fmt = logging.Formatter("%(asctime)s %(levelname)s %(message)s")

    stream_handler = logging.StreamHandler()
    stream_handler.setFormatter(fmt)
    logger.addHandler(stream_handler)

    log_filename = log_filename or f"{name}.log"
    try:
        file_handler = logging.FileHandler(os.path.join(base_dir, log_filename))
        file_handler.setFormatter(fmt)
        logger.addHandler(file_handler)
    except OSError as e:
        # No debería tumbar el pipeline si el directorio de logs no es
        # escribible (p.ej. permisos); se sigue con solo el handler de consola.
        logger.warning(f"No se pudo abrir el archivo de log {log_filename}: {e}")

    return logger
