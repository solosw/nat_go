package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"

	"awesomeProject/internal/tunnel"
)

// HandleSSERequest 处理SSE请求
func HandleSSERequest(targetURL string, msg *tunnel.Message, tunnelConn *tunnel.Tunnel, writer http.ResponseWriter) error {
	// 构建目标URL
	fullURL := targetURL + msg.Path
	
	// 创建HTTP请求
	req, err := http.NewRequest(msg.Method, fullURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
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
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	// 设置SSE响应头
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")
	
	// 刷新响应头
	flusher, ok := writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("响应不支持流式传输")
	}
	flusher.Flush()
	
	// 读取SSE流并转发
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		
		// 将SSE数据发送到服务端
		sseMsg := &tunnel.Message{
			Type:    tunnel.MessageTypeSSE,
			ID:      msg.ID,
			SSEData: line,
		}
		
		if err := tunnelConn.SendMessage(sseMsg); err != nil {
			return fmt.Errorf("发送SSE数据失败: %v", err)
		}
	}
	
	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("读取SSE流失败: %v", err)
	}
	
	return nil
}

// ForwardSSEToClient 服务端转发SSE到客户端
func ForwardSSEToClient(writer http.ResponseWriter, tunnelConn *tunnel.Tunnel, requestID string) error {
	// 设置SSE响应头
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")
	
	flusher, ok := writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("响应不支持流式传输")
	}
	
	// 监听SSE消息
	for {
		msg, err := tunnelConn.ReadMessage()
		if err != nil {
			return fmt.Errorf("读取消息失败: %v", err)
		}
		
		// 检查是否是当前请求的SSE消息
		if msg.Type == tunnel.MessageTypeSSE && msg.ID == requestID {
			// 写入SSE数据
			if _, err := writer.Write([]byte(msg.SSEData + "\n")); err != nil {
				return fmt.Errorf("写入SSE数据失败: %v", err)
			}
			flusher.Flush()
		} else if msg.Type == tunnel.MessageTypeError && msg.ID == requestID {
			// 错误消息
			return fmt.Errorf("SSE错误: %s", msg.Error)
		} else if msg.Type == tunnel.MessageTypePong {
			// 心跳响应
			tunnelConn.UpdatePing()
			continue
		}
	}
}

// IsSSERequest 判断是否是SSE请求
func IsSSERequest(headers map[string][]string) bool {
	accept := headers["Accept"]
	for _, val := range accept {
		if strings.Contains(val, "text/event-stream") {
			return true
		}
	}
	return false
}

// IsWebSocketRequest 判断是否是WebSocket请求
func IsWebSocketRequest(headers map[string][]string) bool {
	connection := headers["Connection"]
	upgrade := headers["Upgrade"]
	
	hasConnection := false
	hasUpgrade := false
	
	for _, val := range connection {
		if strings.ToLower(val) == "upgrade" {
			hasConnection = true
			break
		}
	}
	
	for _, val := range upgrade {
		if strings.ToLower(val) == "websocket" {
			hasUpgrade = true
			break
		}
	}
	
	return hasConnection && hasUpgrade
}

