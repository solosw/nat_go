package tunnel

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Tunnel 隧道连接
type Tunnel struct {
	ID            string
	Conn          *websocket.Conn
	LastPing      time.Time
	responseChans map[string]chan *Message // requestID -> response channel
	mu            sync.RWMutex
}

// NewTunnel 创建新的隧道连接
func NewTunnel(id string, conn *websocket.Conn) *Tunnel {
	return &Tunnel{
		ID:            id,
		Conn:          conn,
		LastPing:      time.Now(),
		responseChans: make(map[string]chan *Message),
	}
}

// Manager 隧道管理器
type Manager struct {
	tunnels  map[string]*Tunnel // tunnelID -> Tunnel
	mu       sync.RWMutex
	upgrader websocket.Upgrader
}

// NewManager 创建隧道管理器
func NewManager() *Manager {
	return &Manager{
		tunnels: make(map[string]*Tunnel),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源
			},
		},
	}
}

// RegisterTunnel 注册隧道
func (m *Manager) RegisterTunnel(tunnelID string, conn *websocket.Conn) *Tunnel {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel := NewTunnel(tunnelID, conn)

	// 如果已存在，关闭旧连接
	if oldTunnel, exists := m.tunnels[tunnelID]; exists {
		oldTunnel.Conn.Close()
	}

	m.tunnels[tunnelID] = tunnel
	log.Printf("隧道注册成功: %s", tunnelID)
	return tunnel
}

// GetTunnel 获取隧道
func (m *Manager) GetTunnel(tunnelID string) (*Tunnel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tunnel, exists := m.tunnels[tunnelID]
	return tunnel, exists
}

// GetSingleTunnelID 返回当下唯一的隧道ID（仅在只存在一个隧道时可用）
func (m *Manager) GetSingleTunnelID() (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.tunnels) != 1 {
		return "", false
	}

	for id := range m.tunnels {
		return id, true
	}

	return "", false
}

// GetFirstTunnelID 返回第一个可用的隧道ID（如果存在）
func (m *Manager) GetFirstTunnelID() (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.tunnels) == 0 {
		return "", false
	}

	for id := range m.tunnels {
		return id, true
	}

	return "", false
}

// RemoveTunnel 移除隧道
func (m *Manager) RemoveTunnel(tunnelID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tunnel, exists := m.tunnels[tunnelID]; exists {
		tunnel.Conn.Close()
		delete(m.tunnels, tunnelID)
		log.Printf("隧道已移除: %s", tunnelID)
	}
}

// SendMessage 发送消息到隧道（线程安全）
func (t *Tunnel) SendMessage(msg *Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	
	return t.Conn.WriteMessage(websocket.TextMessage, data)
}

// RegisterResponseChan 注册响应通道
func (t *Tunnel) RegisterResponseChan(requestID string) chan *Message {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch := make(chan *Message, 1)
	t.responseChans[requestID] = ch
	return ch
}

// UnregisterResponseChan 注销响应通道
func (t *Tunnel) UnregisterResponseChan(requestID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if ch, exists := t.responseChans[requestID]; exists {
		close(ch)
		delete(t.responseChans, requestID)
	}
}

// DispatchMessage 分发消息到对应的响应通道
func (t *Tunnel) DispatchMessage(msg *Message) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if ch, exists := t.responseChans[msg.ID]; exists {
		select {
		case ch <- msg:
		default:
		}
	}
}

// StartMessageDispatcher 启动消息分发器
func (t *Tunnel) StartMessageDispatcher() {
	go func() {
		for {
			msg, err := t.ReadMessage()
			if err != nil {
				log.Printf("读取消息失败，关闭隧道 %s: %v", t.ID, err)
				return
			}

			// 处理心跳
			if msg.Type == MessageTypePong {
				t.UpdatePing()
				continue
			}

			// 分发消息
			t.DispatchMessage(msg)
		}
	}()
}

// ReadMessage 从隧道读取消息
func (t *Tunnel) ReadMessage() (*Message, error) {
	_, data, err := t.Conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	var msg Message
	err = json.Unmarshal(data, &msg)
	if err != nil {
		return nil, err
	}

	return &msg, nil
}

// UpdatePing 更新心跳时间
func (t *Tunnel) UpdatePing() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.LastPing = time.Now()
}

// StartHeartbeat 启动心跳检测
func (m *Manager) StartHeartbeat() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			m.mu.RLock()
			tunnels := make([]*Tunnel, 0, len(m.tunnels))
			for _, tunnel := range m.tunnels {
				tunnels = append(tunnels, tunnel)
			}
			m.mu.RUnlock()

			for _, tunnel := range tunnels {
				// 发送ping
				pingMsg := &Message{Type: MessageTypePing}
				if err := tunnel.SendMessage(pingMsg); err != nil {
					log.Printf("发送心跳失败，移除隧道 %s: %v", tunnel.ID, err)
					m.RemoveTunnel(tunnel.ID)
					continue
				}

				// 检查超时（60秒未收到pong）
				tunnel.mu.RLock()
				timeout := time.Since(tunnel.LastPing) > 60*time.Second
				tunnel.mu.RUnlock()

				if timeout {
					log.Printf("隧道心跳超时，移除隧道 %s", tunnel.ID)
					m.RemoveTunnel(tunnel.ID)
				}
			}
		}
	}()
}
