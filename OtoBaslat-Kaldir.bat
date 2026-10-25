@echo off
chcp 65001 >nul
net session >nul 2>&1
if %errorLevel% neq 0 (
    powershell -NoProfile -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

schtasks /Delete /TN "FlowKVM_AnaPC" /F >nul 2>&1
schtasks /Delete /TN "FlowKVM_DigerPC" /F >nul 2>&1
set "STARTUP=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup"
del /F /Q "%STARTUP%\FlowKVM-AnaPC.lnk" >nul 2>&1
del /F /Q "%STARTUP%\FlowKVM-DigerPC.lnk" >nul 2>&1
echo [TAMAM] FlowKVM Windows otomatik baslangictan tamamen kaldirildi.
pause
