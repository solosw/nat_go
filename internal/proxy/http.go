package proxy

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"awesomeProject/internal/tunnel"
)

// HTTPProxy HTTP代理
type HTTPProxy struct {
	manager *tunnel.Manager
	client  *http.Client
}

// NewHTTPProxy 创建HTTP代理
func NewHTTPProxy(manager *tunnel.Manager) *HTTPProxy {
	return &HTTPProxy{
		manager: manager,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ForwardRequest 转发HTTP请求
func (p *HTTPProxy) ForwardRequest(tunnelID string, msg *tunnel.Message) (*tunnel.Message, error) {
	// 获取隧道连接
	tunnelConn, exists := p.manager.GetTunnel(tunnelID)
	if !exists {
		return &tunnel.Message{
			Type:  tunnel.MessageTypeError,
			ID:    msg.ID,
			Error: "隧道不存在",
		}, nil
	}

	// 注册响应通道
	responseChan := tunnelConn.RegisterResponseChan(msg.ID)
	defer tunnelConn.UnregisterResponseChan(msg.ID)
	
	// 发送请求到客户端
	err := tunnelConn.SendMessage(msg)
	if err != nil {
		return &tunnel.Message{
			Type:  tunnel.MessageTypeError,
			ID:    msg.ID,
			Error: "发送请求失败: " + err.Error(),
		}, nil
	}

	// 等待响应（设置超时）
	timeout := time.After(30 * time.Second)
	
	// 等待响应或超时
	select {
	case respMsg := <-responseChan:
		return respMsg, nil
	case <-timeout:
		return &tunnel.Message{
			Type:  tunnel.MessageTypeError,
			ID:    msg.ID,
			Error: "请求超时",
		}, nil
	}
}

// HandleClientRequest 客户端处理请求（转发到本地服务）
func HandleClientRequest(targetURL string, msg *tunnel.Message) (*tunnel.Message, error) {
	// 构建目标URL
	fullURL := targetURL + msg.Path
	
	// 创建HTTP请求
	req, err := http.NewRequest(msg.Method, fullURL, bytes.NewReader(msg.Body))
	if err != nil {
		return &tunnel.Message{
			Type:  tunnel.MessageTypeError,
			ID:    msg.ID,
			Error: "创建请求失败: " + err.Error(),
		}, nil
	}
	
	// 设置请求头
	for key, values := range msg.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	
	// 发送请求
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return &tunnel.Message{
			Type:  tunnel.MessageTypeError,
			ID:    msg.ID,
			Error: "请求失败: " + err.Error(),
		}, nil
	}
	defer resp.Body.Close()
	
	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &tunnel.Message{
			Type:  tunnel.MessageTypeError,
			ID:    msg.ID,
			Error: "读取响应失败: " + err.Error(),
		}, nil
	}
	
	// 构建响应消息
	responseMsg := &tunnel.Message{
		Type:    tunnel.MessageTypeResponse,
		ID:      msg.ID,
		Status:  resp.StatusCode,
		Headers: make(map[string][]string),
		Body:    body,
	}
	
	// 复制响应头
	for key, values := range resp.Header {
		responseMsg.Headers[key] = values
	}
	
	return responseMsg, nil
}

