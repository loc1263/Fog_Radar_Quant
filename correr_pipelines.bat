@echo off
REM Corre todos los pipelines en secuencia (Chile, EEUU, Tendencia, ETF) con el backend Go.
REM Otras opciones: correr_pipelines.bat --offline   |   --no-cache   |   etf
REM (los argumentos se pasan tal cual a "fogradar job"; sin argumentos corre --todos)
cd /d "%~dp0backend-go"
if not exist bin\fogradar.exe (
  set CGO_ENABLED=0
  go build -trimpath -o bin\fogradar.exe .\cmd\fogradar || ( pause & exit /b 1 )
)
if "%~1"=="" ( bin\fogradar.exe job --todos ) else ( bin\fogradar.exe job %* )
pause
