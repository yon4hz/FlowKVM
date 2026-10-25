@echo off
chcp 65001 >nul
title FlowKVM - Diger Bilgisayar (Client)
cd /d "%~dp0"

:loop
FlowKVM-DigerPC.exe -mode client -ip auto
timeout /t 2 >nul
goto loop
