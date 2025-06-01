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

// type WSMessageData struct {
// 	Code    int    `json:"code"`
// 	Message string `json:"message"`
// 	Data    any    `json:"data"`
// }

// // WebSocket 相关类型定义
type WSMessage struct {
	Type      WSMSG_TYPE     `json:"type"`
	Data      any            `json:"data,omitempty"`
	Timestamp int64          `json:"timestamp"`
	ClientID  string         `json:"client_id,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type WSBroadcastMessage struct {
	Message any
	Group   string
}

type WSHandler func(*gin.Context, any) (any, error)

type WebSocketServer struct {
	ctx         context.Context
	log         *logrus.Entry
	upgrader    websocket.Upgrader
	connections map[string]*websocket.Conn
	mu          sync.RWMutex
	broadcast   chan WSBroadcastMessage
	simpleCache *SimpleMessageCache
}

func NewWebSocketServer(ctx context.Context, addr, version string, log *logrus.Entry) *WebSocketServer {
	return &WebSocketServer{
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
		broadcast:   make(chan WSBroadcastMessage, 100),
	}
}

func (s *WebSocketServer) SendText(clientID string, msg string) error {
	s.mu.RLock()
	conn, exists := s.connections[clientID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}
	return conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

func (s *WebSocketServer) SendJSON(clientID string, msg any) error {
	s.mu.RLock()
	conn, exists := s.connections[clientID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}
	return conn.WriteJSON(msg)
}

func (s *WebSocketServer) Broadcast(msg any) {
	s.broadcast <- WSBroadcastMessage{Message: msg}
}

// handleWSBroadcast 处理 WebSocket 广播消息
func (s *WebSocketServer) handleBroadcast() {
	for {
		select {
		case broadcastMsg := <-s.broadcast:
			s.mu.RLock()
			for clientID, conn := range s.connections {
				if err := conn.WriteJSON(broadcastMsg.Message); err != nil {
					// 移除失效连接
					delete(s.connections, clientID)
				}
			}
			s.mu.RUnlock()
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
	return true
}

// 生成消息Hash
func (s *WebSocketServer) generateMessageHash(msg any, clientID string) string {
	switch msg.(type) {
	case WSMessage:
		msg := msg.(WSMessage)
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
