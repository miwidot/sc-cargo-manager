@echo off
rem Signiert sc-cargo-manager.exe mit dem Code-Signing-Zertifikat aus dem User-Store.
cd /d "%~dp0"

rem signtool suchen (neueste SDK-Version)
set "SIGNTOOL="
for /f "delims=" %%D in ('dir /b /ad /o-n "C:\Program Files (x86)\Windows Kits\10\bin\10.*" 2^>nul') do (
  if not defined SIGNTOOL if exist "C:\Program Files (x86)\Windows Kits\10\bin\%%D\x64\signtool.exe" set "SIGNTOOL=C:\Program Files (x86)\Windows Kits\10\bin\%%D\x64\signtool.exe"
)
if not defined SIGNTOOL ( echo signtool.exe nicht gefunden ^(Windows SDK^). & if not "%1"=="nopause" pause & exit /b 1 )

"%SIGNTOOL%" sign /sha1 9738D348735DCEFE13BAEEB1477B05315FC58767 /fd SHA256 /tr http://time.certum.pl /td SHA256 sc-cargo-manager.exe
if errorlevel 1 ( echo Signieren fehlgeschlagen. & if not "%1"=="nopause" pause & exit /b 1 )
echo Signiert.
if not "%1"=="nopause" pause
