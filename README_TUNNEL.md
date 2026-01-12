# 内网穿透工具使用说明

这是一个简单的内网穿透工具，类似于natapp，支持HTTP请求转发和SSE（Server-Sent Events）流式传输。

## 功能特性

- ✅ HTTP/HTTPS请求转发
- ✅ SSE（Server-Sent Events）支持
- ✅ 多隧道支持（一个服务端可管理多个客户端）
- ✅ 自动心跳检测和重连
- ✅ 无需认证（简化版）

## 架构说明

### 组件

1. **服务端（Server）**：运行在公网服务器上，接收外部HTTP请求
2. **客户端（Client）**：运行在内网机器上，连接到服务端并转发请求到本地服务

### 工作流程

```
外部用户 → 公网服务端 → WebSocket长连接 → 内网客户端 → 本地服务
         ←            ←                  ←            ←
```

## 安装依赖

```bash
go get github.com/gorilla/websocket
go get github.com/gin-gonic/gin
```

## 使用方法

### 1. 配置服务端

编辑 `configs/server.yaml` 配置文件：

```yaml
tunnel_server:
  port: 8080          # 服务端监听端口
  read_timeout: 60    # 读取超时（秒）
  write_timeout: 60   # 写入超时（秒）
```

### 2. 启动服务端

服务端运行在公网服务器上：

```bash
go run cmd/server/main.go
```

**注意**：可以通过环境变量 `TUNNEL_SERVER_CONFIG` 指定配置文件路径：
```bash
export TUNNEL_SERVER_CONFIG=/path/to/custom/server.yaml
go run cmd/server/main.go
```

服务端启动后会显示：
```
配置加载成功: NatappServer v1.0.0
服务端端口: 8080
内网穿透服务端启动在端口 :8080
```

### 3. 配置客户端

编辑 `configs/client.yaml` 配置文件：

```yaml
tunnel_client:
  server_url: "ws://公网服务器IP:8080/ws"  # 服务端WebSocket地址
  tunnel_id: ""                              # 隧道ID（可选，留空则自动生成）
  target_url: "http://localhost:8080"       # 目标本地服务地址
```

配置说明：
- `server_url`: 服务端WebSocket地址，格式为 `ws://IP:端口/ws`
- `tunnel_id`: 隧道ID（可选），留空则服务端自动生成
- `target_url`: 目标本地服务地址，客户端会将请求转发到此地址

### 4. 启动客户端

客户端运行在内网机器上：

```bash
go run cmd/client/main.go
```

**注意**：可以通过环境变量 `TUNNEL_CLIENT_CONFIG` 指定配置文件路径：
```bash
export TUNNEL_CLIENT_CONFIG=/path/to/custom/client.yaml
go run cmd/client/main.go
```

客户端连接成功后会显示：
```
配置加载成功: NatappClient v1.0.0
连接到服务端: ws://公网服务器IP:8080/ws
目标服务地址: http://localhost:8080
已连接到服务端
隧道注册成功，隧道ID: tunnel-xxxxx
外部访问地址: http://服务端地址/tunnel/tunnel-xxxxx/你的路径
```

### 3. 访问内网服务

假设：
- 服务端地址：`http://example.com:8080`
- 隧道ID：`tunnel-abc123`
- 内网服务：`http://localhost:8080/api/users`

外部访问地址为：
```
http://example.com:8080/tunnel/tunnel-abc123/api/users
```

## 示例

### 示例1：HTTP API转发

1. 配置并启动服务端：
```bash
# 编辑 configs/server.yaml，设置端口
go run cmd/server/main.go
```

2. 配置并启动客户端（假设本地服务运行在3000端口）：
```bash
# 编辑 configs/client.yaml：
#   server_url: "ws://your-server.com:8080/ws"
#   target_url: "http://localhost:3000"
go run cmd/client/main.go
```

3. 访问内网API：
```bash
curl http://your-server.com:8080/tunnel/tunnel-xxxxx/api/users
```

### 示例2：SSE流式传输

如果内网服务提供SSE端点（如 `/events`），客户端会自动识别并转发SSE流：

```bash
# 访问SSE端点
curl -N http://your-server.com:8080/tunnel/tunnel-xxxxx/events
```

## 注意事项

1. **安全性**：当前版本没有认证机制，仅适用于测试环境
2. **性能**：每个隧道使用一个WebSocket连接，支持并发请求
3. **超时**：HTTP请求超时时间为30秒，SSE连接超时为5分钟
4. **心跳**：每30秒发送一次心跳，60秒未响应会自动断开

## 项目结构

```
natapp/
├── cmd/
│   ├── server/          # 服务端程序
│   │   └── main.go
│   └── client/          # 客户端程序
│       └── main.go
├── internal/
│   ├── tunnel/          # 隧道管理
│   │   ├── manager.go   # 连接管理器
│   │   └── protocol.go  # 通信协议
│   └── proxy/           # 代理转发
│       ├── http.go      # HTTP转发
│       └── sse.go       # SSE转发
└── README_TUNNEL.md     # 使用说明
```

## 通信协议

使用JSON消息格式通过WebSocket通信：

```json
{
  "type": "request",
  "id": "req-123",
  "method": "GET",
  "path": "/api/users",
  "headers": {...},
  "body": "..."
}
```

消息类型：
- `register`: 客户端注册
- `request`: 请求消息（服务端→客户端）
- `response`: 响应消息（客户端→服务端）
- `sse`: SSE事件消息
- `ping/pong`: 心跳消息

## 故障排查

1. **连接失败**：检查服务端是否启动，网络是否可达
2. **请求超时**：检查内网服务是否正常运行
3. **隧道不存在**：检查客户端是否已连接，隧道ID是否正确

## 开发计划

- [ ] 添加认证机制
- [ ] 支持HTTPS
- [ ] 添加Web管理界面
- [ ] 支持TCP/UDP转发
- [ ] 添加日志和监控

