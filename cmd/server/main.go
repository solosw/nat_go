package main

import (
	"awesomeProject/internal/common"
	"awesomeProject/internal/proxy"
	"awesomeProject/internal/tunnel"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var tunnelManager *tunnel.Manager
var httpProxy *proxy.HTTPProxy
var isPrivateUse bool
var tcpPort int
var tcpConns sync.Map // connID -> net.Conn

func main() {
	// 加载配置文件（可通过环境变量覆盖）
	configPath := os.Getenv("TUNNEL_SERVER_CONFIG")
	if configPath == "" {
		configPath = "./configs/server.yaml"
	}
	config, err := common.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 检查服务端配置
	if config.TunnelServer.Port == 0 {
		log.Fatalf("配置文件中 tunnel_server.port 未设置")
	}

	log.Printf("配置加载成功: %s v%s", config.App.Name, config.App.Version)
	log.Printf("服务端端口: %d", config.TunnelServer.Port)

	// 设置Gin模式
	if config.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化隧道管理器
	tunnelManager = tunnel.NewManager()
	httpProxy = proxy.NewHTTPProxy(tunnelManager)
	isPrivateUse = config.TunnelServer.PrivateUse
	tcpPort = config.TunnelServer.TCPPort

	// 启动心跳检测
	tunnelManager.StartHeartbeat()

	// 创建Gin路由器
	router := gin.Default()

	// WebSocket连接端点（客户端连接）- 必须在通配符路由之前
	router.GET("/ws", handleWebSocket)

	// 健康检查 - 必须在通配符路由之前
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 根据配置决定是否启用多隧道路由
	if !config.TunnelServer.PrivateUse {
		// HTTP代理端点（外部请求）- 多隧道场景
		router.Any("/tunnel/:tunnelID/*path", handleProxyRequest)
		log.Println("多隧道模式已启用，支持 /tunnel/{隧道ID}/ 前缀访问")
	} else {
		log.Println("私人使用模式已启用，仅支持直接路径访问（无需 /tunnel/ 前缀）")
	}

	// 单隧道场景下的简化访问（使用 NoRoute 处理未匹配的路由）
	router.NoRoute(handleDefaultProxyRequest)

	// 启动TCP穿透监听
	if tcpPort > 0 {
		go startTCPListener(tcpPort)
		log.Printf("TCP穿透监听端口: %d", tcpPort)
	} else {
		log.Println("TCP穿透未开启，如需开启请配置 tunnel_server.tcp_port")
	}

	// 启动服务器
	port := fmt.Sprintf(":%d", config.TunnelServer.Port)
	log.Printf("内网穿透服务端启动在端口 %s", port)
	if err := router.Run(port); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

// handleWebSocket 处理WebSocket连接（客户端连接）
func handleWebSocket(c *gin.Context) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}
	defer conn.Close()

	log.Println("新的WebSocket连接")

	// 等待客户端注册消息
	var tunnelID string
	for {
		var msg tunnel.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("读取消息失败: %v", err)
			return
		}

		if msg.Type == tunnel.MessageTypeRegister {
			tunnelID = msg.TunnelID
			if tunnelID == "" {
				// 如果没有提供tunnelID，生成一个
				tunnelID = generateTunnelID()
			}

			// 注册隧道
			tunnelConn := tunnelManager.RegisterTunnel(tunnelID, conn)

			// 发送注册成功消息
			response := tunnel.Message{
				Type:     tunnel.MessageTypeResponse,
				TunnelID: tunnelID,
			}
			conn.WriteJSON(response)

			log.Printf("隧道注册成功: %s", tunnelID)

			// 启动消息分发器
			tunnelConn.StartMessageDispatcher()
			break
		} else if msg.Type == tunnel.MessageTypePong {
			// 心跳响应
			if tunnelConn, exists := tunnelManager.GetTunnel(msg.TunnelID); exists {
				tunnelConn.UpdatePing()
			}
		}
	}

	// 保持连接
	select {}
}

// handleProxyRequest 处理代理请求（外部HTTP请求）
func handleProxyRequest(c *gin.Context) {
	tunnelID := c.Param("tunnelID")
	path := c.Param("path")
	processProxyRequest(c, tunnelID, path)
}

// handleDefaultProxyRequest 处理无隧道前缀的代理请求（单隧道场景）
// 作为 NoRoute 处理器，处理所有未匹配的路由
func handleDefaultProxyRequest(c *gin.Context) {
	// 从请求URL获取路径
	path := c.Request.URL.Path
	
	// 确保路径以 / 开头
	if path == "" {
		path = "/"
	}

	var tunnelID string
	var ok bool

	if isPrivateUse {
		// 私人使用模式：使用第一个可用隧道
		tunnelID, ok = tunnelManager.GetFirstTunnelID()
		if !ok {
			c.JSON(503, gin.H{"error": "没有可用的隧道连接"})
			return
		}
	} else {
		// 多隧道模式：仅当存在唯一隧道时才允许省略前缀
		tunnelID, ok = tunnelManager.GetSingleTunnelID()
		if !ok {
			c.JSON(400, gin.H{"error": "请使用 /tunnel/{隧道ID}/ 前缀访问或仅保持一个隧道连接"})
			return
		}
	}

	processProxyRequest(c, tunnelID, path)
}

// processProxyRequest 统一的代理处理逻辑
func processProxyRequest(c *gin.Context, tunnelID, path string) {
	// 获取隧道连接
	tunnelConn, exists := tunnelManager.GetTunnel(tunnelID)
	if !exists {
		c.JSON(503, gin.H{"error": "隧道不存在或未连接"})
		return
	}

	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(500, gin.H{"error": "读取请求体失败"})
		return
	}

	// 构建请求消息
	requestID := generateRequestID()
	// 将查询参数附加到路径上
	fullPath := path
	if c.Request.URL.RawQuery != "" {
		fullPath = path + "?" + c.Request.URL.RawQuery
	}
	msg := &tunnel.Message{
		Type:    tunnel.MessageTypeRequest,
		ID:      requestID,
		Method:  c.Request.Method,
		Path:    fullPath,
		Headers: c.Request.Header,
		Body:    body,
	}

	// 检查是否是SSE请求
	if proxy.IsSSERequest(c.Request.Header) {
		handleSSEProxy(c, tunnelConn, msg)
		return
	}

	// 检查是否是WebSocket请求
	if proxy.IsWebSocketRequest(c.Request.Header) {
		proxy.HandleWebSocketProxy(c, tunnelConn, requestID, fullPath)
		return
	}

	// 转发HTTP请求
	respMsg, err := httpProxy.ForwardRequest(tunnelID, msg)
	if err != nil {
		c.JSON(500, gin.H{"error": "转发请求失败: " + err.Error()})
		return
	}

	if respMsg.Type == tunnel.MessageTypeError {
		c.JSON(500, gin.H{"error": respMsg.Error})
		return
	}

	// 设置响应头
	for key, values := range respMsg.Headers {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// 返回响应
	c.Data(respMsg.Status, c.GetHeader("Content-Type"), respMsg.Body)
}

// handleSSEProxy 处理SSE代理请求
func handleSSEProxy(c *gin.Context, tunnelConn *tunnel.Tunnel, msg *tunnel.Message) {
	// 发送请求到客户端
	err := tunnelConn.SendMessage(msg)
	if err != nil {
		c.JSON(500, gin.H{"error": "发送请求失败: " + err.Error()})
		return
	}

	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(500, gin.H{"error": "响应不支持流式传输"})
		return
	}

	flusher.Flush()

	// 注册SSE响应通道
	responseChan := tunnelConn.RegisterResponseChan(msg.ID)
	defer tunnelConn.UnregisterResponseChan(msg.ID)

	// 监听SSE消息
	timeout := time.After(5 * time.Minute) // SSE超时5分钟
	for {
		select {
		case <-timeout:
			return
		case respMsg := <-responseChan:
			// 检查是否是当前请求的SSE消息
			if respMsg.Type == tunnel.MessageTypeSSE && respMsg.ID == msg.ID {
				// 写入SSE数据
				c.Writer.Write([]byte(respMsg.SSEData + "\n"))
				flusher.Flush()
			} else if respMsg.Type == tunnel.MessageTypeError && respMsg.ID == msg.ID {
				// 错误消息
				c.Writer.Write([]byte("event: error\ndata: " + respMsg.Error + "\n\n"))
				flusher.Flush()
				return
			}
		}
	}
}

// startTCPListener 启动TCP穿透监听
func startTCPListener(port int) {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("启动TCP监听失败: %v", err)
		return
	}

	for {
		publicConn, err := ln.Accept()
		if err != nil {
			log.Printf("接受TCP连接失败: %v", err)
			continue
		}
		go handleTCPConnection(publicConn)
	}
}

// handleTCPConnection 处理单个TCP连接
func handleTCPConnection(publicConn net.Conn) {
	tunnelID, ok := selectTunnelIDForTCP()
	if !ok {
		log.Printf("无可用隧道，拒绝TCP连接来自 %s", publicConn.RemoteAddr().String())
		publicConn.Close()
		return
	}

	tunnelConn, exists := tunnelManager.GetTunnel(tunnelID)
	if !exists {
		log.Printf("隧道 %s 不存在，拒绝TCP连接", tunnelID)
		publicConn.Close()
		return
	}

	connID := generateRequestID()

	// 注册响应通道
	responseChan := tunnelConn.RegisterResponseChan(connID)
	defer tunnelConn.UnregisterResponseChan(connID)

	// 发送TCP初始化消息
	initMsg := &tunnel.Message{
		Type: tunnel.MessageTypeTCPInit,
		ID:   connID,
	}
	if err := tunnelConn.SendMessage(initMsg); err != nil {
		log.Printf("发送TCP初始化失败: %v", err)
		publicConn.Close()
		return
	}

	// 从公网读取数据并转发给内网
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := publicConn.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				dataMsg := &tunnel.Message{
					Type: tunnel.MessageTypeTCPData,
					ID:   connID,
					Body: data,
				}
				if err := tunnelConn.SendMessage(dataMsg); err != nil {
					log.Printf("发送TCP数据失败: %v", err)
					break
				}
			}
			if err != nil {
				closeMsg := &tunnel.Message{
					Type: tunnel.MessageTypeTCPClose,
					ID:   connID,
				}
				tunnelConn.SendMessage(closeMsg)
				break
			}
		}
	}()

	// 从内网读取数据并转发给公网
	for {
		msg, ok := <-responseChan
		if !ok {
			break
		}

		switch msg.Type {
		case tunnel.MessageTypeTCPData:
			if _, err := publicConn.Write(msg.Body); err != nil {
				log.Printf("写入公网TCP失败: %v", err)
				publicConn.Close()
				return
			}
		case tunnel.MessageTypeTCPClose, tunnel.MessageTypeError:
			publicConn.Close()
			return
		}
	}
}

// selectTunnelIDForTCP 选择用于TCP穿透的隧道ID
func selectTunnelIDForTCP() (string, bool) {
	if isPrivateUse {
		return tunnelManager.GetFirstTunnelID()
	}
	return tunnelManager.GetSingleTunnelID()
}

// generateTunnelID 生成隧道ID
func generateTunnelID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "tunnel-" + hex.EncodeToString(bytes)
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "req-" + hex.EncodeToString(bytes)
}
