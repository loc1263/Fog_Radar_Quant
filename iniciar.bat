@echo off
REM Lanza iniciar.py (backend Go :8000 + frontend SvelteKit :5173). Acepta los mismos argumentos,
REM p. ej.: iniciar.bat --rebuild --no-browser
cd /d "%~dp0"

set "PY="
where py >nul 2>nul && set "PY=py -3"
if not defined PY (
  where python >nul 2>nul && set "PY=python"
)
if not defined PY (
  echo ERROR: no se encontro Python 3. Instalalo desde https://www.python.org/downloads/
  pause
  exit /b 1
)

%PY% iniciar.py %*
set "RC=%errorlevel%"
if not "%RC%"=="0" (
  echo.
  echo El lanzador termino con errores (codigo %RC%).
  pause
)
exit /b %RC%
