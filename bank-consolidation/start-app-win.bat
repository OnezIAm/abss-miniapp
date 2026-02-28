@echo off
cd /d "%~dp0"

:: Set the port to 8585 to match the frontend configuration
set ADDR=:8585

echo ---------------------------------------------------
echo Starting Bank Consolidation App...
echo Backend Port: %ADDR%
echo ---------------------------------------------------

:: Start the backend in a separate window
start "Bank Consolidation Backend" bank-app-win.exe

:: Wait a few seconds for the server to initialize
timeout /t 3

:: Open the default web browser
echo Opening browser...
start http://localhost:8585
