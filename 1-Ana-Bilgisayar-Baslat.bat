@echo off
chcp 65001 >nul
title FlowKVM - Ana Bilgisayar (Server)
cd /d "%~dp0"

:loop
FlowKVM-AnaPC.exe -mode server -edge left -ip auto
timeout /t 2 >nul
goto loop
