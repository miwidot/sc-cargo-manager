@echo off
title SC Cargo Manager
cd /d "%~dp0"

if not exist "sc-cargo-manager.exe" (
  echo Baue sc-cargo-manager.exe ...
  go build -ldflags="-H windowsgui -s -w" -o sc-cargo-manager.exe .
  if errorlevel 1 (
    echo.
    echo FEHLER beim Bauen. Ist Go installiert? ^(https://go.dev/dl^)
    pause
    exit /b 1
  )
)

start "" "sc-cargo-manager.exe"
