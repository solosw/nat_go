package proxy

import (
	"log"
	"net/http"
	"strings"
	"time"

	"awesomeProject/internal/tunnel"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// HandleWebSocketProxy 服务端处理WebSocket代理请求
func HandleWebSocketProxy(c *gin.Context, tunnelConn *tunnel.Tunnel, requestID string, path string) {
	// 构建请求消息
	msg := &tunnel.Message{
		Type:    tunnel.MessageTypeWebSocket,
		ID:      requestID,
		Method:  c.Request.Method,
		Path:    path,
		Headers: c.Request.Header,
		Body:    nil,
	}

	// 发送WebSocket升级请求到客户端
	err := tunnelConn.SendMessage(msg)
	if err != nil {
		c.JSON(500, gin.H{"error": "发送WebSocket请求失败: " + err.Error()})
		return
	}

	// 注册响应通道
	responseChan := tunnelConn.RegisterResponseChan(requestID)
	defer tunnelConn.UnregisterResponseChan(requestID)

	// 等待客户端响应（WebSocket升级响应）
	timeout := time.After(10 * time.Second)
	var wsRespMsg *tunnel.Message
	select {
	case respMsg := <-responseChan:
		if respMsg.Type == tunnel.MessageTypeError {
			c.JSON(500, gin.H{"error": respMsg.Error})
			return
		}
		wsRespMsg = respMsg
	case <-timeout:
		c.JSON(500, gin.H{"error": "WebSocket升级超时"})
		return
	}

	// 检查响应状态码
	if wsRespMsg.Status != 101 {
		c.JSON(wsRespMsg.Status, gin.H{"error": "WebSocket升级失败"})
		return
	}

	// 升级当前连接为WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket升级失败: %v", err)
		return
	}
	defer conn.Close()

	// 启动双向数据转发
	done := make(chan struct{})

	// 从外部客户端读取，转发到内网客户端
	go func() {
		defer close(done)
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				if !websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket读取错误: %v", err)
				}
				return
			}

			// 发送数据到内网客户端
			wsDataMsg := &tunnel.Message{
				Type:          tunnel.MessageTypeWebSocketData,
				ID:            requestID,
				WSData:        data,
				WSMessageType: messageType,
			}
			if err := tunnelConn.SendMessage(wsDataMsg); err != nil {
				log.Printf("发送WebSocket数据失败: %v", err)
				return
			}
		}
	}()

	// 从内网客户端读取，转发到外部客户端
	for {
		select {
		case <-done:
			return
		case respMsg := <-responseChan:
			if respMsg.Type == tunnel.MessageTypeWebSocketData && respMsg.ID == requestID {
				// 转发数据到外部客户端
				if err := conn.WriteMessage(respMsg.WSMessageType, respMsg.WSData); err != nil {
					log.Printf("写入WebSocket数据失败: %v", err)
					return
				}
			} else if respMsg.Type == tunnel.MessageTypeError && respMsg.ID == requestID {
				log.Printf("WebSocket错误: %s", respMsg.Error)
				return
			}
		}
	}
}

// HandleClientWebSocket 客户端处理WebSocket请求
func HandleClientWebSocket(targetURL string, msg *tunnel.Message, tunnelConn *tunnel.Tunnel) {
	// 构建目标WebSocket URL
	wsURL := targetURL + msg.Path
	// 将 http:// 或 https:// 转换为 ws:// 或 wss://
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)

	// 构建请求头
	headers := make(http.Header)
	for key, values := range msg.Headers {
		// 过滤掉一些不应该转发的头
		if key == "Connection" || key == "Upgrade" || key == "Sec-Websocket-Key" ||
			key == "Sec-Websocket-Version" || key == "Sec-Websocket-Extensions" ||
			key == "Sec-Websocket-Protocol" {
			continue
		}
		for _, value := range values {
			headers.Add(key, value)
		}
	}

	// 连接到目标WebSocket服务器
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		errorMsg := &tunnel.Message{
			Type:  tunnel.MessageTypeError,
			ID:    msg.ID,
			Error: "连接WebSocket失败: " + err.Error(),
		}
		tunnelConn.SendMessage(errorMsg)
		return
	}
	defer conn.Close()

	// 发送WebSocket升级成功响应
	responseMsg := &tunnel.Message{
		Type:   tunnel.MessageTypeResponse,
		ID:     msg.ID,
		Status: 101, // Switching Protocols
		Headers: make(map[string][]string),
	}
	// 复制响应头（如果需要）
	responseMsg.Headers["Upgrade"] = []string{"websocket"}
	responseMsg.Headers["Connection"] = []string{"Upgrade"}
	tunnelConn.SendMessage(responseMsg)

	// 启动双向数据转发
	done := make(chan struct{})

	// 从内网服务读取，转发到服务端
	go func() {
		defer close(done)
		for {
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				if !websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket读取错误: %v", err)
				}
				return
			}

			// 发送数据到服务端
			wsDataMsg := &tunnel.Message{
				Type:          tunnel.MessageTypeWebSocketData,
				ID:            msg.ID,
				WSData:        data,
				WSMessageType: messageType,
			}
			if err := tunnelConn.SendMessage(wsDataMsg); err != nil {
				log.Printf("发送WebSocket数据失败: %v", err)
				return
			}
		}
	}()

	// 从服务端读取WebSocket数据，转发到内网服务
	// 通过消息分发器接收数据（在handleRequests中处理）
	// 这里需要持续监听消息分发器发送的WebSocket数据消息
	// 由于消息分发器已经在handleRequests中处理，我们需要一个特殊的处理机制
	// 实际上，WebSocket数据应该通过专门的消息通道传递
	// 这里我们使用一个goroutine来持续监听responseChan
	responseChan := tunnelConn.RegisterResponseChan(msg.ID)
	defer tunnelConn.UnregisterResponseChan(msg.ID)

	for {
		select {
		case <-done:
			return
		case respMsg := <-responseChan:
			if respMsg.Type == tunnel.MessageTypeWebSocketData && respMsg.ID == msg.ID {
				// 转发数据到内网服务
				if err := conn.WriteMessage(respMsg.WSMessageType, respMsg.WSData); err != nil {
					log.Printf("写入WebSocket数据失败: %v", err)
					return
				}
			} else if respMsg.Type == tunnel.MessageTypeError && respMsg.ID == msg.ID {
				log.Printf("WebSocket错误: %s", respMsg.Error)
				return
			}
		}
	}
}
