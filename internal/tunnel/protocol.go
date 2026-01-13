package tunnel

// MessageType 消息类型
type MessageType string

const (
	// MessageTypeRegister 客户端注册
	MessageTypeRegister MessageType = "register"
	// MessageTypeRequest 请求消息（服务端 -> 客户端）
	MessageTypeRequest MessageType = "request"
	// MessageTypeResponse 响应消息（客户端 -> 服务端）
	MessageTypeResponse MessageType = "response"
	// MessageTypeSSE  SSE事件消息（客户端 -> 服务端）
	MessageTypeSSE MessageType = "sse"
	// MessageTypeTCPInit TCP连接初始化
	MessageTypeTCPInit MessageType = "tcp_init"
	// MessageTypeTCPData TCP数据传输
	MessageTypeTCPData MessageType = "tcp_data"
	// MessageTypeTCPClose TCP连接关闭
	MessageTypeTCPClose MessageType = "tcp_close"
	// MessageTypeWebSocket WebSocket消息
	MessageTypeWebSocket MessageType = "websocket"
	// MessageTypeWebSocketData WebSocket数据消息
	MessageTypeWebSocketData MessageType = "websocket_data"
	// MessageTypeError 错误消息
	MessageTypeError MessageType = "error"
	// MessageTypePing 心跳消息
	MessageTypePing MessageType = "ping"
	// MessageTypePong 心跳响应
	MessageTypePong MessageType = "pong"
)

// Message 通信消息结构
type Message struct {
	Type    MessageType          `json:"type"`
	ID      string               `json:"id,omitempty"`      // 请求ID，用于匹配请求和响应
	TunnelID string              `json:"tunnel_id,omitempty"` // 隧道ID
	Method  string               `json:"method,omitempty"`  // HTTP方法
	Path    string               `json:"path,omitempty"`    // 请求路径
	Headers map[string][]string `json:"headers,omitempty"` // HTTP头
	Body    []byte               `json:"body,omitempty"`    // 请求/响应体
	Status      int    `json:"status,omitempty"`        // HTTP状态码
	Error       string `json:"error,omitempty"`         // 错误信息
	SSEData     string `json:"sse_data,omitempty"`      // SSE数据
	WSData      []byte `json:"ws_data,omitempty"`       // WebSocket数据
	WSMessageType int  `json:"ws_message_type,omitempty"` // WebSocket消息类型（1=Text, 2=Binary）
}

