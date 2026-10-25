@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo [*] FlowKVM derleniyor...

go test -v .
if %errorLevel% neq 0 (
    echo [!] Testler basarisiz oldu!
    pause
    exit /b 1
)

go build -o FlowKVM-AnaPC.exe .
go build -ldflags="-H=windowsgui" -o FlowKVM-AnaPC-Gizli.exe .
go build -tags clientonly -o FlowKVM-DigerPC.exe .
go build -tags clientonly -ldflags="-H=windowsgui" -o FlowKVM-DigerPC-Gizli.exe .
go build -o FlowKVM.exe .

echo [✓] Tum .exe dosyalari basariyla derlendi!
pause
