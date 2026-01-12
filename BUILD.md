# 构建说明

本文档说明如何构建内网穿透工具的服务端和客户端。

## 快速开始

### Windows 环境

#### 方式1：使用批处理脚本（推荐）

```bash
build.bat
```

#### 方式2：使用 PowerShell 脚本

```powershell
.\build.ps1
```

#### 方式3：手动构建

**构建 Linux 服务端：**
```bash
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -ldflags "-s -w" -o build/server/natapp-server-linux-amd64 cmd/server/main.go
```

**构建 Linux ARM64 服务端：**
```bash
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=arm64
go build -ldflags "-s -w" -o build/server/natapp-server-linux-arm64 cmd/server/main.go
```

**构建 Windows 客户端：**
```bash
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-s -w" -o build/client/natapp-client-windows-amd64.exe cmd/client/main.go
```

### Linux 环境

使用 Makefile：

```bash
make all          # 构建所有版本
make server-linux # 只构建 Linux 服务端
make client-windows # 只构建 Windows 客户端
make clean        # 清理构建文件
```

或手动构建：

**构建 Linux 服务端：**
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o build/server/natapp-server-linux-amd64 cmd/server/main.go
```

**构建 Windows 客户端：**
```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o build/client/natapp-client-windows-amd64.exe cmd/client/main.go
```

## 构建输出

构建完成后，文件将输出到以下目录：

```
build/
├── server/
│   ├── natapp-server-linux-amd64      # Linux 服务端 (AMD64)
│   ├── natapp-server-linux-arm64      # Linux 服务端 (ARM64)
│   └── server.yaml                    # 服务端配置文件
└── client/
    ├── natapp-client-windows-amd64.exe # Windows 客户端
    └── client.yaml                      # 客户端配置文件
```

## 部署说明

### Linux 服务端部署

1. 将 `natapp-server-linux-amd64` 上传到 Linux 服务器
2. 添加执行权限：
   ```bash
   chmod +x natapp-server-linux-amd64
   ```
3. 配置 `server.yaml` 文件
4. 运行：
   ```bash
   ./natapp-server-linux-amd64
   ```

### Windows 客户端部署

1. 将 `natapp-client-windows-amd64.exe` 复制到 Windows 机器
2. 配置 `client.yaml` 文件
3. 双击运行或使用命令行：
   ```bash
   natapp-client-windows-amd64.exe
   ```

## 交叉编译说明

Go 支持交叉编译，可以在一个平台上编译其他平台的二进制文件。

### 支持的平台

- **服务端**：
  - Linux AMD64 (x86_64)
  - Linux ARM64 (aarch64)

- **客户端**：
  - Windows AMD64 (x86_64)

### 环境变量说明

- `CGO_ENABLED=0`：禁用 CGO，确保静态链接，提高兼容性
- `GOOS`：目标操作系统（linux, windows, darwin 等）
- `GOARCH`：目标架构（amd64, arm64, 386 等）

### 链接参数说明

- `-s`：省略符号表和调试信息
- `-w`：省略 DWARF 符号表
- 这些参数可以减小二进制文件大小

## 常见问题

### 1. 构建失败：找不到包

确保已安装所有依赖：
```bash
go mod download
go mod tidy
```

### 2. Linux 版本无法执行

确保添加了执行权限：
```bash
chmod +x natapp-server-linux-amd64
```

### 3. 文件太大

使用 `-ldflags "-s -w"` 可以减小文件大小，或者使用 UPX 压缩：
```bash
upx --best natapp-server-linux-amd64
```

## 版本信息

构建的二进制文件包含版本信息，可以通过以下方式查看：

```bash
# Linux
./natapp-server-linux-amd64 --version

# Windows
natapp-client-windows-amd64.exe --version
```

（注意：当前版本未实现 --version 参数，可以在代码中添加）

