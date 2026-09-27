@echo off
REM Levanta backend (FastAPI :8000) y frontend (SvelteKit :5173) juntos.
REM Ctrl+C en esta ventana detiene ambos.
cd /d "%~dp0"
where python >nul 2>nul
if errorlevel 1 (
  echo [ERROR] No se encontro Python en el PATH.
  pause
  exit /b 1
)
python iniciar_dev.py %*
if errorlevel 1 pause
