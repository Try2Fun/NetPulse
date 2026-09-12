@echo off
color 0b
title NetPulse Launcher
echo ==========================================
echo        Iniciando NetPulse...
echo ==========================================
echo.

:: Cambiar al directorio donde está el script
cd /d "%~dp0"

:: Abrir el navegador en el dashboard
echo Abriendo el dashboard en el navegador...
start http://localhost:8080

:: Ejecutar el programa con escaneo automático y web server
echo Ejecutando el motor de NetPulse...
echo.
go run ./cmd/netpulse -scan -save -web

:: Si el programa se cierra o crashea, pausar para ver el error
pause
