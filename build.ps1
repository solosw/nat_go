# 内网穿透工具构建脚本 (PowerShell)
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "内网穿透工具构建脚本" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# 设置版本号
$VERSION = "1.0.0"
$BUILD_TIME = Get-Date -Format "yyyyMMdd_HHmmss"

# 创建输出目录
if (-not (Test-Path "build")) { New-Item -ItemType Directory -Path "build" | Out-Null }
if (-not (Test-Path "build\server")) { New-Item -ItemType Directory -Path "build\server" | Out-Null }
if (-not (Test-Path "build\client")) { New-Item -ItemType Directory -Path "build\client" | Out-Null }

Write-Host "[1/4] 构建 Linux 服务端..." -ForegroundColor Yellow
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$ldflags = "-X main.Version=$VERSION -X main.BuildTime=$BUILD_TIME -s -w"
go build -ldflags $ldflags -o build\server\natapp-server-linux-amd64 cmd\server\main.go
if ($LASTEXITCODE -ne 0) {
    Write-Host "构建 Linux 服务端失败！" -ForegroundColor Red
    exit 1
}
Write-Host "✓ Linux 服务端构建成功: build\server\natapp-server-linux-amd64" -ForegroundColor Green

Write-Host ""
Write-Host "[2/4] 构建 Linux ARM64 服务端..." -ForegroundColor Yellow
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "arm64"
go build -ldflags $ldflags -o build\server\natapp-server-linux-arm64 cmd\server\main.go
if ($LASTEXITCODE -ne 0) {
    Write-Host "构建 Linux ARM64 服务端失败！" -ForegroundColor Red
    exit 1
}
Write-Host "✓ Linux ARM64 服务端构建成功: build\server\natapp-server-linux-arm64" -ForegroundColor Green

Write-Host ""
Write-Host "[3/4] 构建 Windows 客户端..." -ForegroundColor Yellow
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -ldflags $ldflags -o build\client\natapp-client-windows-amd64.exe cmd\client\main.go
if ($LASTEXITCODE -ne 0) {
    Write-Host "构建 Windows 客户端失败！" -ForegroundColor Red
    exit 1
}
Write-Host "✓ Windows 客户端构建成功: build\client\natapp-client-windows-amd64.exe" -ForegroundColor Green

Write-Host ""
Write-Host "[4/4] 复制配置文件..." -ForegroundColor Yellow
Copy-Item -Path "configs\server.yaml" -Destination "build\server\server.yaml" -Force | Out-Null
Copy-Item -Path "configs\client.yaml" -Destination "build\client\client.yaml" -Force | Out-Null
Write-Host "✓ 配置文件已复制" -ForegroundColor Green

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "构建完成！" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "输出文件：" -ForegroundColor Yellow
Write-Host "  服务端 (Linux):   build\server\natapp-server-linux-amd64" -ForegroundColor White
Write-Host "  服务端 (ARM64):   build\server\natapp-server-linux-arm64" -ForegroundColor White
Write-Host "  客户端 (Windows): build\client\natapp-client-windows-amd64.exe" -ForegroundColor White
Write-Host ""

