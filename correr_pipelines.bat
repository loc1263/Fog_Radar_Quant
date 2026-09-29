@echo off
REM Corre todos los pipelines en secuencia (Chile, EEUU, Tendencia, ETF).
REM Opciones: correr_pipelines.bat --offline   |   correr_pipelines.bat --solo etf
cd /d "%~dp0"
python scripts\run_all_pipelines.py %*
pause
