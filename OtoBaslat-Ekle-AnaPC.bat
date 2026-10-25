@echo off
chcp 65001 >nul
net session >nul 2>&1
if %errorLevel% neq 0 (
    powershell -NoProfile -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

cd /d "%~dp0"

:: 1. Windows Gorev Zamanlayiciya En Yuksek Yetkiyle (Yonetici + Penceresiz + UAC sormadan) ekle
schtasks /Create /TN "FlowKVM_AnaPC" /TR "\"%~dp0FlowKVM-AnaPC-Gizli.exe\"" /SC ONLOGON /RL HIGHEST /F >nul 2>&1

:: 2. Yedek olarak Baslangic (Startup) klasorune de ekle
set "STARTUP=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup"
powershell -NoProfile -Command "$ws = New-Object -ComObject WScript.Shell; $s = $ws.CreateShortcut('%STARTUP%\FlowKVM-AnaPC.lnk'); $s.TargetPath = '%~dp0FlowKVM-AnaPC-Gizli.exe'; $s.WorkingDirectory = '%~dp0'; $s.Save()" >nul 2>&1

:: Simdi de arka planda hemen baslat
taskkill /F /IM FlowKVM-AnaPC-Gizli.exe >nul 2>&1
start "" "%~dp0FlowKVM-AnaPC-Gizli.exe"

echo ================================================================
echo [TAMAM] ANA BILGISAYAR OTOMATIK BASLATMA KURULDU VE SIMDI BASLATILDI!
echo Bilgisayari her actiginizda hicbir pencere veya onay sormadan
echo arka planda Yonetici yetkisiyle otomatik calisacaktir.
echo ================================================================
pause
