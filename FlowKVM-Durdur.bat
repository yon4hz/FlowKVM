@echo off
chcp 65001 >nul
net session >nul 2>&1
if %errorLevel% neq 0 (
    powershell -NoProfile -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

cd /d "%~dp0"
taskkill /F /IM FlowKVM-AnaPC-Gizli.exe >nul 2>&1
taskkill /F /IM FlowKVM-AnaPC-Gizli.old.exe >nul 2>&1
taskkill /F /IM FlowKVM-DigerPC-Gizli.exe >nul 2>&1
taskkill /F /IM FlowKVM-AnaPC.exe >nul 2>&1
taskkill /F /IM FlowKVM-AnaPC.old.exe >nul 2>&1
taskkill /F /IM FlowKVM-AnaPC.old2.exe >nul 2>&1
taskkill /F /IM FlowKVM-DigerPC.exe >nul 2>&1
taskkill /F /IM FlowKVM.exe >nul 2>&1
del /F /Q "%~dp0*.old*.exe" >nul 2>&1
echo [TAMAM] Arka planda calisan tum FlowKVM islemleri kapatildi.
timeout /t 2 >nul
