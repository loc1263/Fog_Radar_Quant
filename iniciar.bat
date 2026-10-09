@echo off
REM Levanta el backend Go (:8000) y el frontend SvelteKit (:5173) en dos ventanas.
REM Cierra cada ventana (o Ctrl+C en ella) para detenerlo.
cd /d "%~dp0"
if not exist backend-go\bin\fogradar.exe (
  echo Compilando el backend...
  pushd backend-go
  set CGO_ENABLED=0
  go build -trimpath -o bin\fogradar.exe .\cmd\fogradar
  if errorlevel 1 ( popd & pause & exit /b 1 )
  popd
)
start "Fog Radar - backend" /D "%~dp0backend-go" cmd /k "bin\fogradar.exe api"
start "Fog Radar - frontend" /D "%~dp0frontend" cmd /k "npm run dev"
