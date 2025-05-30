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
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type WSMSG_TYPE string

const (
	WSMSG_TYPE_WELCOME WSMSG_TYPE = "welcome"
	WSMSG_TYPE_PING    WSMSG_TYPE = "ping"
)

// WebSocket 相关类型定义
type WSMessage struct {
	Type      WSMSG_TYPE     `json:"type"`
	Data      any            `json:"data"`
	Timestamp int64          `json:"timestamp"`
	ClientID  string         `json:"client_id,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type WSBroadcastMessage struct {
	Message WSMessage
	Group   string
}

type WSHandler func(*gin.Context, *websocket.Conn, *WSMessage)

type WebSocketServer struct {
	ctx         context.Context
	log         *logrus.Entry
	upgrader    websocket.Upgrader
	connections map[string]*websocket.Conn
	mu          sync.RWMutex
	broadcast   chan WSBroadcastMessage
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

func (s *WebSocketServer) send(conn *websocket.Conn, msg WSMessage) error {
	return conn.WriteJSON(msg)
}

func (s *WebSocketServer) Send(clientID string, msg WSMessage) error {
	s.mu.RLock()
	conn, exists := s.connections[clientID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}
	return s.send(conn, msg)
}

func (s *WebSocketServer) Broadcast(msg WSMessage) {
	s.broadcast <- WSBroadcastMessage{Message: msg}
}

// handleWSBroadcast 处理 WebSocket 广播消息
func (s *WebSocketServer) handleBroadcast() {
	for {
		select {
		case broadcastMsg := <-s.broadcast:
			s.mu.RLock()
			for clientID, conn := range s.connections {
				if err := s.send(conn, broadcastMsg.Message); err != nil {
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
