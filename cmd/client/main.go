package main

import (
	"awesomeProject/internal/common"
	"awesomeProject/internal/proxy"
	"awesomeProject/internal/tunnel"
	"bufio"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var (
	serverURL  string
	tunnelID   string
	targetURL  string
	tcpTarget  string
	conn       *websocket.Conn
	tunnelConn *tunnel.Tunnel
	tcpConns   sync.Map // connID -> net.Conn
)

func main() {
	// 加载配置文件（可通过环境变量覆盖）
	configPath := os.Getenv("TUNNEL_CLIENT_CONFIG")
	if configPath == "" {
		configPath = "./configs/client.yaml"
	}
	config, err := common.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 检查客户端配置
	if config.TunnelClient.ServerURL == "" {
		log.Fatalf("配置文件中 tunnel_client.server_url 未设置")
	}
	if config.TunnelClient.TargetURL == "" {
		log.Fatalf("配置文件中 tunnel_client.target_url 未设置")
	}

	serverURL = config.TunnelClient.ServerURL
	tunnelID = config.TunnelClient.TunnelID
	targetURL = config.TunnelClient.TargetURL
	tcpTarget = config.TunnelClient.TCPTarget

	log.Printf("配置加载成功: %s v%s", config.App.Name, config.App.Version)
	log.Printf("连接到服务端: %s", serverURL)
	log.Printf("目标服务地址: %s", targetURL)
	if tunnelID != "" {
		log.Printf("使用隧道ID: %s", tunnelID)
	}

	// 连接到服务端
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err = dialer.Dial(serverURL, nil)
	if err != nil {
		log.Fatalf("连接服务端失败: %v", err)
	}
	defer conn.Close()

	log.Println("已连接到服务端")

	// 注册隧道
	registerMsg := tunnel.Message{
		Type:     tunnel.MessageTypeRegister,
		TunnelID: tunnelID,
	}

	err = conn.WriteJSON(registerMsg)
	if err != nil {
		log.Fatalf("发送注册消息失败: %v", err)
	}

	// 等待注册响应
	var registerResp tunnel.Message
	err = conn.ReadJSON(&registerResp)
	if err != nil {
		log.Fatalf("读取注册响应失败: %v", err)
	}

	if registerResp.TunnelID != "" {
		tunnelID = registerResp.TunnelID
		log.Printf("隧道注册成功，隧道ID: %s", tunnelID)
		log.Printf("外部访问地址: http://服务端地址/你的路径（单隧道默认）")
		log.Printf("多隧道场景访问: http://服务端地址/tunnel/%s/你的路径", tunnelID)
	}

	// 创建隧道连接对象
	tunnelConn = tunnel.NewTunnel(tunnelID, conn)

	// 启动心跳
	go startHeartbeat()

	// 启动请求处理
	go handleRequests()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("正在关闭连接...")
}

// startHeartbeat 启动心跳
func startHeartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		pingMsg := tunnel.Message{
			Type:     tunnel.MessageTypePong,
			TunnelID: tunnelID,
		}

		if err := tunnelConn.SendMessage(&pingMsg); err != nil {
			log.Printf("发送心跳失败: %v", err)
			return
		}
	}
}

// handleRequests 处理来自服务端的请求
func handleRequests() {
	for {
		var msg tunnel.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket连接关闭: %v", err)
			}
			return
		}

		// 处理心跳
		if msg.Type == tunnel.MessageTypePing {
			pongMsg := tunnel.Message{
				Type:     tunnel.MessageTypePong,
				TunnelID: tunnelID,
			}
			tunnelConn.SendMessage(&pongMsg)
			tunnelConn.UpdatePing()
			continue
		}

		// 处理请求
		if msg.Type == tunnel.MessageTypeRequest {
			go handleRequest(&msg)
		}

		// 处理TCP隧道初始化
		if msg.Type == tunnel.MessageTypeTCPInit {
			go handleTCPInit(&msg)
		}

		// 处理TCP数据（必须保持顺序，不能开goroutine）
		if msg.Type == tunnel.MessageTypeTCPData {
			handleTCPData(&msg)
		}

		// 处理TCP关闭
		if msg.Type == tunnel.MessageTypeTCPClose {
			handleTCPClose(&msg)
		}

		// 处理WebSocket请求
		if msg.Type == tunnel.MessageTypeWebSocket {
			go handleWebSocketRequest(&msg)
		}

		// 处理WebSocket数据消息
		if msg.Type == tunnel.MessageTypeWebSocketData {
			// WebSocket数据消息通过消息分发器处理
			tunnelConn.DispatchMessage(&msg)
		}
	}
}

// handleRequest 处理单个请求
func handleRequest(msg *tunnel.Message) {
	// 检查是否是SSE请求
	if proxy.IsSSERequest(msg.Headers) {
		handleSSERequest(msg)
		return
	}

	// 处理普通HTTP请求
	respMsg, err := proxy.HandleClientRequest(targetURL, msg)
	if err != nil {
		errorMsg := tunnel.Message{
			Type:  tunnel.MessageTypeError,
			ID:    msg.ID,
			Error: err.Error(),
		}
		tunnelConn.SendMessage(&errorMsg)
		return
	}

	// 发送响应
	err = tunnelConn.SendMessage(respMsg)
	if err != nil {
		log.Printf("发送响应失败: %v", err)
	}
}

// handleTCPInit 处理TCP隧道初始化
func handleTCPInit(msg *tunnel.Message) {
	if tcpTarget == "" {
		errMsg := &tunnel.Message{
			Type:  tunnel.MessageTypeError,
			ID:    msg.ID,
			Error: "客户端未配置 tcp_target，无法建立TCP隧道",
		}
		tunnelConn.SendMessage(errMsg)
		return
	}

	localConn, err := net.Dial("tcp", tcpTarget)
	if err != nil {
		errMsg := &tunnel.Message{
			Type:  tunnel.MessageTypeError,
			ID:    msg.ID,
			Error: "连接本地TCP失败: " + err.Error(),
		}
		tunnelConn.SendMessage(errMsg)
		return
	}

	tcpConns.Store(msg.ID, localConn)

	// 将本地TCP的数据转发到服务端
	go func(connID string, c net.Conn) {
		buf := make([]byte, 32*1024)
		for {
			n, err := c.Read(buf)
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
		tcpConns.Delete(connID)
		c.Close()
	}(msg.ID, localConn)
}

// handleTCPData 将服务端的数据写入本地TCP连接
func handleTCPData(msg *tunnel.Message) {
	if v, ok := tcpConns.Load(msg.ID); ok {
		if c, ok2 := v.(net.Conn); ok2 {
			if _, err := c.Write(msg.Body); err != nil {
				log.Printf("写入本地TCP失败: %v", err)
				handleTCPClose(msg)
			}
		}
	}
}

// handleTCPClose 关闭本地TCP连接
func handleTCPClose(msg *tunnel.Message) {
	if v, ok := tcpConns.Load(msg.ID); ok {
		if c, ok2 := v.(net.Conn); ok2 {
			c.Close()
		}
		tcpConns.Delete(msg.ID)
	}
}

// handleSSERequest 处理SSE请求
func handleSSERequest(msg *tunnel.Message) {
	// 构建目标URL
	fullURL := targetURL + msg.Path

	// 创建HTTP请求
	req, err := http.NewRequest(msg.Method, fullURL, nil)
	if err != nil {
		errorMsg := tunnel.Message{
			Type:  tunnel.MessageTypeError,
			ID:    msg.ID,
			Error: "创建请求失败: " + err.Error(),
		}
		tunnelConn.SendMessage(&errorMsg)
		return
	}

	// 设置请求头
	for key, values := range msg.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// 设置SSE相关请求头
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// 发送请求
	client := &http.Client{
		Timeout: 0, // SSE是长连接，不设置超时
	}

	resp, err := client.Do(req)
	if err != nil {
		errorMsg := tunnel.Message{
			Type:  tunnel.MessageTypeError,
			ID:    msg.ID,
			Error: "请求失败: " + err.Error(),
		}
		tunnelConn.SendMessage(&errorMsg)
		return
	}
	defer resp.Body.Close()

	// 使用bufio.Scanner按行读取SSE流
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 设置缓冲区大小

	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			sseMsg := tunnel.Message{
				Type:    tunnel.MessageTypeSSE,
				ID:      msg.ID,
				SSEData: line,
			}

			if err := tunnelConn.SendMessage(&sseMsg); err != nil {
				log.Printf("发送SSE数据失败: %v", err)
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("读取SSE流失败: %v", err)
		errorMsg := tunnel.Message{
			Type:  tunnel.MessageTypeError,
			ID:    msg.ID,
			Error: "读取SSE流失败: " + err.Error(),
		}
		tunnelConn.SendMessage(&errorMsg)
	}
}

// handleWebSocketRequest 处理WebSocket请求
func handleWebSocketRequest(msg *tunnel.Message) {
	proxy.HandleClientWebSocket(targetURL, msg, tunnelConn)
}
