@echo off
title FlowKVM - GitHub Yukleyici
set "GIT_DIR=C:\Users\YUNUS EREN\AppData\Local\Programs\GitTools\git\cmd"
set "GH_DIR=C:\Users\YUNUS EREN\AppData\Local\Programs\GitTools\gh\bin"
set "PATH=%GIT_DIR%;%GH_DIR%;%SystemRoot%\system32;%SystemRoot%"
cd /d "C:\Users\YUNUS EREN\FlowKVM"
echo ================================================================
echo             FlowKVM - GitHub Otomatik Yukleyici
echo ================================================================
echo.
"%GH_DIR%\gh.exe" auth status >nul 2>&1
if errorlevel 1 (
    echo [*] GitHub hesabiniza giris yapmaniz gerekiyor.
    echo [*] Simdi Enter tusuna bastiginizda tarayicinizda acilacak sayfaya ekrandaki 8 haneli kodu girin:
    echo.
    "%GH_DIR%\gh.exe" auth login --hostname github.com --git-protocol https --web
)
echo.
echo [*] FlowKVM deposu (Public) olusturuluyor ve dosyalar yukleniyor...
"%GH_DIR%\gh.exe" repo create FlowKVM --public --source=. --remote=origin --push
if errorlevel 1 (
    echo [*] Mevcut depoya gonderiliyor (git push)...
    "%GIT_DIR%\git.exe" push -u origin main
)
echo.
echo ================================================================
echo [TAMAM] Islem tamamlandi! GitHub deponuz tarayicida aciliyor...
echo ================================================================
"%GH_DIR%\gh.exe" repo view --web
pause
