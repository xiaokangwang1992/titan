/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2025/05/28 16:01:04
 Desc     : websocket service
*/

package service

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type WSMSG_TYPE string

const (
	WSMSG_TYPE_WELCOME WSMSG_TYPE = "welcome"
	WSMSG_TYPE_PING    WSMSG_TYPE = "ping"

	WSMSG_TYPE_SUCCESS WSMSG_TYPE = "success"
	WSMSG_TYPE_ERROR   WSMSG_TYPE = "error"
)

// 服务端事件类型
type ServerEventType int

const (
	ServerEventClientConnected    ServerEventType = iota // 客户端连接
	ServerEventClientDisconnected                        // 客户端断开
	ServerEventMessageReceived                           // 收到消息
	ServerEventMessageSent                               // 发送消息
	ServerEventBroadcastSent                             // 广播消息
	ServerEventError                                     // 发生错误
)

// 服务端事件
type ServerEvent struct {
	Type      ServerEventType
	ClientID  string
	Data      any
	Error     error
	Timestamp time.Time
	Metadata  map[string]any
}

// 事件处理器
type ServerEventHandler func(event ServerEvent)

// type WSMessageData struct {
// 	Code    int    `json:"code"`
// 	Message string `json:"message"`
// 	Data    any    `json:"data"`
// }

// // WebSocket 相关类型定义
type WSMessage[T any] struct {
	Type      WSMSG_TYPE     `json:"type"`
	Data      T              `json:"data,omitempty"`
	Timestamp int64          `json:"timestamp"`
	ClientID  string         `json:"client_id,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type WSBroadcastMessage[T any] struct {
	Message T
	Group   string
}

type WSHandler[T any] func(*gin.Context, T) (any, error)

type WebSocketServer struct {
	ctx         context.Context
	log         *logrus.Entry
	upgrader    websocket.Upgrader
	connections map[string]*websocket.Conn
	mu          sync.RWMutex
	broadcast   chan WSBroadcastMessage[any]
	simpleCache *SimpleMessageCache

	// 事件处理相关
	eventChan    chan ServerEvent
	handlers     map[ServerEventType][]ServerEventHandler
	handlersMu   sync.RWMutex
	ignoreEvents map[ServerEventType]bool
}

func NewWebSocketServer(ctx context.Context, addr, version string, log *logrus.Entry) *WebSocketServer {
	server := &WebSocketServer{
		ctx: ctx,
		log: log,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		connections: make(map[string]*websocket.Conn),
		mu:          sync.RWMutex{},
		broadcast:   make(chan WSBroadcastMessage[any], 100),
		simpleCache: NewSimpleMessageCache(5 * time.Minute),
		eventChan:   make(chan ServerEvent, 100),
		handlers:    make(map[ServerEventType][]ServerEventHandler),
		handlersMu:  sync.RWMutex{},
		ignoreEvents: map[ServerEventType]bool{
			ServerEventMessageReceived:    true,
			ServerEventMessageSent:        true,
			ServerEventBroadcastSent:      true,
			ServerEventError:              true,
			ServerEventClientConnected:    true,
			ServerEventClientDisconnected: true,
		},
	}
	return server
}

// 注册事件处理器
func (s *WebSocketServer) OnEvent(eventType ServerEventType, handler ServerEventHandler) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.ignoreEvents[eventType] = false
	s.handlers[eventType] = append(s.handlers[eventType], handler)
}

// 发送事件
func (s *WebSocketServer) emitEvent(event ServerEvent) {
	if s.ignoreEvents[event.Type] {
		return
	}
	select {
	case s.eventChan <- event:
	default:
		s.log.Warn("event queue is full, dropping event")
	}
}

// 事件处理循环
func (s *WebSocketServer) eventLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case event := <-s.eventChan:
			s.handlersMu.RLock()
			handlers := s.handlers[event.Type]
			s.handlersMu.RUnlock()

			for _, handler := range handlers {
				go handler(event) // 异步处理避免阻塞
			}
		}
	}
}

// 添加连接时触发事件
func (s *WebSocketServer) addConnection(clientID string, conn *websocket.Conn) {
	s.mu.Lock()
	s.connections[clientID] = conn
	s.mu.Unlock()

	s.emitEvent(ServerEvent{
		Type:      ServerEventClientConnected,
		ClientID:  clientID,
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"total_connections": len(s.connections),
		},
	})
}

// 移除连接时触发事件
func (s *WebSocketServer) removeConnection(clientID string) {
	s.mu.Lock()
	delete(s.connections, clientID)
	totalConnections := len(s.connections)
	s.mu.Unlock()

	s.emitEvent(ServerEvent{
		Type:      ServerEventClientDisconnected,
		ClientID:  clientID,
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"total_connections": totalConnections,
		},
	})
}

func (s *WebSocketServer) SendText(clientID string, msg string) error {
	s.mu.RLock()
	conn, exists := s.connections[clientID]
	s.mu.RUnlock()

	if !exists {
		err := fmt.Errorf("client %s not found", clientID)
		s.emitEvent(ServerEvent{
			Type:      ServerEventError,
			ClientID:  clientID,
			Error:     err,
			Timestamp: time.Now(),
		})
		return err
	}

	err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		s.emitEvent(ServerEvent{
			Type:      ServerEventError,
			ClientID:  clientID,
			Error:     err,
			Timestamp: time.Now(),
		})
	} else {
		s.emitEvent(ServerEvent{
			Type:      ServerEventMessageSent,
			ClientID:  clientID,
			Data:      msg,
			Timestamp: time.Now(),
		})
	}

	return err
}

func (s *WebSocketServer) SendJSON(clientID string, msg any) error {
	s.mu.RLock()
	conn, exists := s.connections[clientID]
	s.mu.RUnlock()

	if !exists {
		err := fmt.Errorf("client %s not found", clientID)
		s.emitEvent(ServerEvent{
			Type:      ServerEventError,
			ClientID:  clientID,
			Error:     err,
			Timestamp: time.Now(),
		})
		return err
	}

	err := conn.WriteJSON(msg)
	if err != nil {
		s.emitEvent(ServerEvent{
			Type:      ServerEventError,
			ClientID:  clientID,
			Error:     err,
			Timestamp: time.Now(),
		})
	} else {
		s.emitEvent(ServerEvent{
			Type:      ServerEventMessageSent,
			ClientID:  clientID,
			Data:      msg,
			Timestamp: time.Now(),
		})
	}

	return err
}

func (s *WebSocketServer) Broadcast(msg any) {
	s.broadcast <- WSBroadcastMessage[any]{Message: msg}
}

// handleWSBroadcast 处理 WebSocket 广播消息
func (s *WebSocketServer) handleBroadcast() {
	for {
		select {
		case broadcastMsg := <-s.broadcast:
			s.mu.RLock()
			successCount := 0
			failCount := 0

			for clientID, conn := range s.connections {
				if err := conn.WriteJSON(broadcastMsg.Message); err != nil {
					// 移除失效连接
					delete(s.connections, clientID)
					failCount++
					s.emitEvent(ServerEvent{
						Type:      ServerEventError,
						ClientID:  clientID,
						Error:     err,
						Timestamp: time.Now(),
					})
				} else {
					successCount++
				}
			}
			s.mu.RUnlock()

			// 触发广播完成事件
			s.emitEvent(ServerEvent{
				Type:      ServerEventBroadcastSent,
				Data:      broadcastMsg.Message,
				Timestamp: time.Now(),
				Metadata: map[string]any{
					"success_count": successCount,
					"fail_count":    failCount,
					"total_clients": successCount + failCount,
				},
			})

		case <-s.ctx.Done():
			return
		}
	}
}

func (s *WebSocketServer) GetClients() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	clients := make([]string, 0, len(s.connections))
	for clientID := range s.connections {
		clients = append(clients, clientID)
	}
	return clients
}

// TODO: 消息预处理器
func (s *WebSocketServer) preprocessMessage(msg any, clientID string) (shouldProcess bool) {
	// // 2. 过滤空消息
	// if msg.Data == nil || msg.Data == "" {
	// 	s.log.Debugf("empty message from %s, ignoring", clientID)
	// 	return false
	// }

	// 3. 过滤重复消息（可选）
	// if s.isDuplicateMessage(msg, clientID) {
	// 	s.log.Debugf("duplicate message from %s, ignoring", clientID)
	// 	return false
	// }
	s.emitEvent(ServerEvent{
		Type:      ServerEventMessageReceived,
		ClientID:  clientID,
		Data:      msg,
		Timestamp: time.Now(),
	})
	return true
}

// 生成消息Hash
func (s *WebSocketServer) generateMessageHash(msg any, clientID string) string {
	switch msg := msg.(type) {
	case WSMessage[any]:
		content := fmt.Sprintf("%s:%v:%s", msg.Type, msg.Data, clientID)
		hash := md5.Sum([]byte(content))
		return fmt.Sprintf("hash_%s_%x", clientID, hash)
	case string:
		content := fmt.Sprintf("%s:%s", msg, clientID)
		hash := md5.Sum([]byte(content))
		return fmt.Sprintf("hash_%s_%x", clientID, hash)
	}
	return ""
}

func (s *WebSocketServer) isDuplicateMessage(msg any, clientID string) bool {
	if msg == nil {
		return false
	}

	// 使用简化的缓存
	cache := s.simpleCache // 需要在ApiServer中添加这个字段

	msgHash := s.generateMessageHash(msg, clientID)

	cache.mu.Lock()
	defer cache.mu.Unlock()

	now := time.Now().Unix()

	// 定期清理过期消息（每分钟清理一次）
	if now-cache.lastClean > 60 {
		cache.cleanExpired(now)
		cache.lastClean = now
	}

	// 检查是否重复
	if lastSeen, exists := cache.messages[msgHash]; exists {
		if now-lastSeen < int64(cache.ttl.Seconds()) {
			return true // 重复消息
		}
	}

	// 记录新消息
	cache.messages[msgHash] = now
	return false
}

// 简化的重复消息检测
type SimpleMessageCache struct {
	mu        sync.RWMutex
	messages  map[string]int64 // hash -> timestamp
	ttl       time.Duration
	lastClean int64
}

func NewSimpleMessageCache(ttl time.Duration) *SimpleMessageCache {
	return &SimpleMessageCache{
		messages:  make(map[string]int64),
		ttl:       ttl,
		lastClean: time.Now().Unix(),
	}
}

func (c *SimpleMessageCache) cleanExpired(now int64) {
	ttlSeconds := int64(c.ttl.Seconds())
	for hash, timestamp := range c.messages {
		if now-timestamp > ttlSeconds {
			delete(c.messages, hash)
		}
	}
}
