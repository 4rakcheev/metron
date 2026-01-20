@echo off
:: Metron Windows Agent Installer
:: Double-click this file to install the agent

echo ========================================
echo Metron Windows Agent Installer
echo ========================================
echo.

:: Check for admin rights
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo Requesting Administrator privileges...
    powershell -Command "Start-Process cmd -ArgumentList '/c cd /d \"%~dp0\" && \"%~f0\"' -Verb RunAs"
    exit /b
)

echo Running as Administrator: OK
echo Current directory: %~dp0
echo.

:: Run PowerShell installer with bypass execution policy
cd /d "%~dp0"
powershell -ExecutionPolicy Bypass -NoProfile -File "%~dp0install.ps1"

echo.
echo ========================================
echo Installation script finished.
echo ========================================
echo.
pause
