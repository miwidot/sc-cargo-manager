@echo off
cd /d "%~dp0"
echo Baue sc-cargo-manager.exe neu ...
go build -ldflags="-H windowsgui -s -w" -o sc-cargo-manager.exe .
if errorlevel 1 ( echo FEHLER. & pause & exit /b 1 )
echo Fertig: sc-cargo-manager.exe
pause
