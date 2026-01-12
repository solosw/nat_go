@echo off
chcp 65001 >nul
echo ========================================
echo 内网穿透工具构建脚本
echo ========================================
echo.

REM 设置版本号
set VERSION=1.0.0
set BUILD_TIME=%date:~0,4%%date:~5,2%%date:~8,2%_%time:~0,2%%time:~3,2%%time:~6,2%
set BUILD_TIME=%BUILD_TIME: =0%

REM 创建输出目录
if not exist "build" mkdir build
if not exist "build\server" mkdir build\server
if not exist "build\client" mkdir build\client

echo [1/4] 构建 Linux 服务端...
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -ldflags "-X main.Version=%VERSION% -X main.BuildTime=%BUILD_TIME% -s -w" -o build\server\natapp-server-linux-amd64.exe cmd\server\main.go
if %errorlevel% neq 0 (
    echo 构建 Linux 服务端失败！
    pause
    exit /b 1
)
echo ✓ Linux 服务端构建成功: build\server\natapp-server-linux-amd64.exe

echo.
echo [2/4] 构建 Linux ARM64 服务端...
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=arm64
go build -ldflags "-X main.Version=%VERSION% -X main.BuildTime=%BUILD_TIME% -s -w" -o build\server\natapp-server-linux-arm64.exe cmd\server\main.go
if %errorlevel% neq 0 (
    echo 构建 Linux ARM64 服务端失败！
    pause
    exit /b 1
)
echo ✓ Linux ARM64 服务端构建成功: build\server\natapp-server-linux-arm64.exe

echo.
echo [3/4] 构建 Windows 客户端...
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-X main.Version=%VERSION% -X main.BuildTime=%BUILD_TIME% -s -w" -o build\client\natapp-client-windows-amd64.exe cmd\client\main.go
if %errorlevel% neq 0 (
    echo 构建 Windows 客户端失败！
    pause
    exit /b 1
)
echo ✓ Windows 客户端构建成功: build\client\natapp-client-windows-amd64.exe

echo.
echo [4/4] 复制配置文件...
copy /Y configs\server.yaml build\server\server.yaml >nul 2>&1
copy /Y configs\client.yaml build\client\client.yaml >nul 2>&1
echo ✓ 配置文件已复制

echo.
echo ========================================
echo 构建完成！
echo ========================================
echo.
echo 输出文件：
echo   服务端 (Linux):   build\server\natapp-server-linux-amd64.exe
echo   服务端 (ARM64):   build\server\natapp-server-linux-arm64.exe
echo   客户端 (Windows): build\client\natapp-client-windows-amd64.exe
echo.
echo 注意：Linux 版本需要重命名为无 .exe 扩展名
echo   例如: natapp-server-linux-amd64.exe -> natapp-server
echo.
pause

