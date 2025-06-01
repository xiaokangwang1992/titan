/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2025/05/30 15:44:34
 Desc     :
*/

package client

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/piaobeizu/titan/service"
	"github.com/sirupsen/logrus"
)

// 连接状态常量
const (
	StateDisconnected int32 = 0
	StateConnecting   int32 = 1
	StateConnected    int32 = 2
	StateClosed       int32 = 3
)

// 配置选项
type Config struct {
	URL               string            // websocket服务器地址
	Headers           map[string]string // 连接头
	PingInterval      time.Duration     // ping间隔，默认30s
	PongTimeout       time.Duration     // pong超时，默认60s
	WriteTimeout      time.Duration     // 写超时，默认10s
	ReadTimeout       time.Duration     // 读超时，默认60s
	ReconnectInterval time.Duration     // 重连间隔，默认3s
	MaxReconnectDelay time.Duration     // 最大重连延迟，默认60s
	ReconnectBackoff  float64           // 重连退避系数，默认1.5
	ReconnectMaxCount int               // 最大重连次数，默认3
	HandshakeTimeout  time.Duration     // 握手超时，默认10s
	BufferSize        int               // 发送缓冲区大小，默认256
	AutoReconnect     bool              // 是否自动重连，默认true
	EnableCompression bool              // 是否启用压缩，默认false
}

// 默认配置
func DefaultConfig(url string) *Config {
	return &Config{
		URL:               url,
		PingInterval:      30 * time.Second,
		PongTimeout:       60 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadTimeout:       60 * time.Second,
		ReconnectInterval: 3 * time.Second,
		MaxReconnectDelay: 60 * time.Second,
		ReconnectBackoff:  1.5,
		ReconnectMaxCount: 3,
		HandshakeTimeout:  10 * time.Second,
		BufferSize:        256,
		AutoReconnect:     true,
		EnableCompression: false,
		Headers:           make(map[string]string),
	}
}

// 事件类型
type EventType int

const (
	EventConnected    EventType = iota // 连接成功
	EventDisconnected                  // 连接断开
	EventMessage                       // 收到消息
	EventError                         // 发生错误
	EventReconnecting                  // 重连中
)

// 事件
type Event struct {
	Type      EventType
	Data      interface{}
	Error     error
	Timestamp time.Time
}

// 事件处理器
type EventHandler func(event Event)

// 高性能websocket客户端
type WSClient struct {
	id     string
	config *Config
	// logger *logrus.Entry

	// 连接相关（使用互斥锁保证并发安全）
	state           int32           // 连接状态
	connMu          sync.RWMutex    // 保护连接操作
	connection      *websocket.Conn // 直接存储连接指针
	lastConnectTime int64           // 上次连接时间（纳秒）

	// 控制通道
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	sendChan  chan any
	eventChan chan Event

	// 事件处理
	handlers   map[EventType][]EventHandler
	handlersMu sync.RWMutex

	// 重连控制
	reconnectDelay int64 // 当前重连延迟（纳秒）
	reconnecting   int32 // 重连状态标志，防止并发重连

	// 性能统计
	statsMu sync.RWMutex // 保护统计信息
	stats   struct {
		sentMessages     int64
		receivedMessages int64
		connectAttempts  int64
		reconnectCount   int64
		lastError        error // 直接存储error
	}
}

// 安全的连接存储
func (c *WSClient) storeConn(conn *websocket.Conn) {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	c.connection = conn
}

// 安全的连接获取
func (c *WSClient) getConn() *websocket.Conn {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connection
}

// 安全地存储错误
func (c *WSClient) storeError(err error) {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	c.stats.lastError = err
}

// 安全地获取错误
func (c *WSClient) getLastError() error {
	c.statsMu.RLock()
	defer c.statsMu.RUnlock()
	return c.stats.lastError
}

// 创建新的websocket客户端
func NewClient(ctx context.Context, id string, config *Config) *WSClient {
	if config == nil {
		config = DefaultConfig("")
	}

	subCtx, cancel := context.WithCancel(ctx)

	// logger := logrus.WithFields(logrus.Fields{
	// 	"action": "websocket",
	// })

	client := &WSClient{
		id:     id,
		config: config,
		// logger:         logger,
		ctx:            subCtx,
		cancel:         cancel,
		done:           make(chan struct{}),
		sendChan:       make(chan any, config.BufferSize),
		eventChan:      make(chan Event, 100),
		handlers:       make(map[EventType][]EventHandler),
		reconnectDelay: int64(config.ReconnectInterval),
		connection:     nil, // 初始化为 nil
	}

	// 启动事件处理器
	go client.eventLoop()

	return client
}

// 连接到服务器
func (c *WSClient) Connect() error {
	if !atomic.CompareAndSwapInt32(&c.state, StateDisconnected, StateConnecting) {
		return fmt.Errorf("client is not in disconnected state")
	}

	atomic.AddInt64(&c.stats.connectAttempts, 1)
	atomic.StoreInt64(&c.lastConnectTime, time.Now().UnixNano())

	conn, err := c.dial()
	if err != nil {
		atomic.StoreInt32(&c.state, StateDisconnected)
		c.storeError(err)
		return err
	}

	// 安全地存储连接
	c.storeConn(conn)
	atomic.StoreInt32(&c.state, StateConnected)

	// 启动读写goroutine
	go c.readLoop()
	go c.writeLoop()
	if c.config.PingInterval > 0 {
		go c.pingLoop()
	}

	c.emitEvent(Event{
		Type:      EventConnected,
		Timestamp: time.Now(),
	})

	logrus.Info("websocket connected successfully")

	// 不再阻塞，立即返回让调用者可以继续执行
	return nil
}

// 等待连接关闭（可选的阻塞方法）
func (c *WSClient) Wait() {
	// wait for signal to exit
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
		return
	case <-c.ctx.Done():
		return
	}
}

// 断开连接
func (c *WSClient) Disconnect() error {
	// 更新状态
	if !atomic.CompareAndSwapInt32(&c.state, StateConnected, StateClosed) &&
		!atomic.CompareAndSwapInt32(&c.state, StateConnecting, StateClosed) {
		return nil // 已经断开或正在断开
	}

	// 获取连接并关闭
	conn := c.getConn()
	if conn != nil {
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = conn.Close()
	}

	// 清空连接
	c.storeConn(nil)

	c.emitEvent(Event{
		Type:      EventDisconnected,
		Timestamp: time.Now(),
	})

	logrus.Info("websocket disconnected")
	return nil
}

// 关闭客户端
func (c *WSClient) Close() error {
	c.cancel()
	close(c.done)
	return c.Disconnect()
}

// 发送消息
func (c *WSClient) SendMessage(msg any) error {
	if atomic.LoadInt32(&c.state) != StateConnected {
		return fmt.Errorf("client is not connected")
	}

	switch msg.(type) {
	case service.WSMessage, *service.WSMessage:
		msg := msg.(service.WSMessage)
		msg.ClientID = c.id
		c.sendChan <- msg
	case string, *string:
		c.sendChan <- msg
	case []byte, *[]byte:
		c.sendChan <- msg
	default:
		return fmt.Errorf("only support service.WSMessage, string, []byte, unsupported message type: %T, %v", msg, msg)
	}
	return nil
}

// 注册事件处理器
func (c *WSClient) OnEvent(eventType EventType, handler EventHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.handlers[eventType] = append(c.handlers[eventType], handler)
}

// 获取连接状态
func (c *WSClient) State() int32 {
	return atomic.LoadInt32(&c.state)
}

func (c *WSClient) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"sent_messages":       atomic.LoadInt64(&c.stats.sentMessages),
		"received_messages":   atomic.LoadInt64(&c.stats.receivedMessages),
		"connect_attempts":    atomic.LoadInt64(&c.stats.connectAttempts),
		"reconnect_count":     atomic.LoadInt64(&c.stats.reconnectCount),
		"max_reconnect_count": c.config.ReconnectMaxCount,
		"state":               c.State(),
	}

	if lastErr := c.getLastError(); lastErr != nil {
		stats["last_error"] = lastErr.Error()
	}

	return stats
}

// 拨号连接
func (c *WSClient) dial() (*websocket.Conn, error) {
	dialer := &websocket.Dialer{
		HandshakeTimeout:  c.config.HandshakeTimeout,
		EnableCompression: c.config.EnableCompression,
	}

	// 设置请求头
	headers := make(http.Header)
	for k, v := range c.config.Headers {
		headers.Set(k, v)
	}

	logrus.Infof("connecting to %s", c.config.URL)

	conn, resp, err := dialer.Dial(c.config.URL, headers)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("dial failed with status %d: %v", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("dial failed: %v", err)
	}

	// 设置读写缓冲区
	if c.config.ReadTimeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	}

	return conn, nil
}

// 读循环
func (c *WSClient) readLoop() {
	defer c.handleDisconnect()

	conn := c.getConn()
	if conn == nil {
		return
	}

	// 设置pong处理器
	if c.config.PongTimeout > 0 {
		conn.SetPongHandler(func(data string) error {
			logrus.Infof("received pong from server: %s", data)
			return conn.SetReadDeadline(time.Now().Add(c.config.PongTimeout))
		})
	}

	logrus.Info("read loop started, listening for messages...")

	for {
		select {
		case <-c.ctx.Done():
			logrus.Info("read loop stopped due to context cancellation")
			return
		default:
			if c.config.ReadTimeout > 0 {
				_ = conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
			}

			messageType, data, err := conn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					logrus.Errorf("read message error: %v", err)
					c.storeError(err)
					c.emitEvent(Event{
						Type:      EventError,
						Error:     err,
						Timestamp: time.Now(),
					})
				}
				return
			}

			logrus.Infof("received message type: %d, data length: %d", messageType, len(data))
			atomic.AddInt64(&c.stats.receivedMessages, 1)

			c.emitEvent(Event{
				Type:      EventMessage,
				Data:      data,
				Timestamp: time.Now(),
			})
		}
	}
}

// 写循环
func (c *WSClient) writeLoop() {
	defer c.handleDisconnect()

	logrus.Info("write loop started, sending messages...")
	conn := c.getConn()
	if conn == nil {
		return
	}
	for {
		select {
		case <-c.ctx.Done():
			return
		case msg := <-c.sendChan:
			if c.config.WriteTimeout > 0 {
				_ = conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
			}
			switch v := msg.(type) {
			case service.WSMessage, *service.WSMessage:
				msg := v.(service.WSMessage)
				if err := conn.WriteJSON(msg); err != nil {
					c.storeError(err)
					c.emitEvent(Event{
						Type:      EventError,
						Error:     err,
						Timestamp: time.Now(),
					})
				}
			case string, *string:
				msg := v.(string)
				if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
					c.storeError(err)
					c.emitEvent(Event{
						Type:      EventError,
						Error:     err,
						Timestamp: time.Now(),
					})
				}
			case []byte, *[]byte:
				msg := v.([]byte)
				if err := conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
					c.storeError(err)
					c.emitEvent(Event{
						Type:      EventError,
						Error:     err,
						Timestamp: time.Now(),
					})
				}
			default:
				c.storeError(fmt.Errorf("only support service.WSMessage, string, []byte, unsupported message type: %T, %v", v, v))
				c.emitEvent(Event{
					Type:      EventError,
					Error:     fmt.Errorf("only support service.WSMessage, string, []byte, unsupported message type: %T, %v", v, v),
					Timestamp: time.Now(),
				})
			}

			atomic.AddInt64(&c.stats.sentMessages, 1)
		}
	}
}

// ping循环
func (c *WSClient) pingLoop() {
	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()
	logrus.Infof("ping loop started with interval: %v", c.config.PingInterval)

	for {
		select {
		case <-c.ctx.Done():
			logrus.Info("ping loop stopped due to context cancellation")
			return
		case <-ticker.C:
			state := atomic.LoadInt32(&c.state)
			if state != StateConnected {
				logrus.Warnf("ping skipped - connection state: %d", state)
				return
			}

			conn := c.getConn()
			if conn == nil {
				logrus.Warn("ping failed - connection is nil")
				return
			}

			// 设置写超时
			if c.config.WriteTimeout > 0 {
				_ = conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
			}

			if err := conn.WriteMessage(websocket.PingMessage, []byte("ping")); err != nil {
				logrus.Errorf("ping failed: %v", err)
				c.storeError(err)
			}
		}
	}
}

// 事件循环
func (c *WSClient) eventLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case event := <-c.eventChan:
			c.handlersMu.RLock()
			handlers := c.handlers[event.Type]
			c.handlersMu.RUnlock()

			for _, handler := range handlers {
				go handler(event) // 异步处理避免阻塞
			}
		}
	}
}

// 处理断开连接
func (c *WSClient) handleDisconnect() {
	if atomic.CompareAndSwapInt32(&c.state, StateConnected, StateDisconnected) {
		c.emitEvent(Event{
			Type:      EventDisconnected,
			Timestamp: time.Now(),
		})

		// 自动重连 - 防止并发重连
		if c.config.AutoReconnect && atomic.CompareAndSwapInt32(&c.reconnecting, 0, 1) {
			go func() {
				defer atomic.StoreInt32(&c.reconnecting, 0)
				c.reconnect()
			}()
		}
	}
}

// 重连逻辑
// 重连逻辑
func (c *WSClient) reconnect() {
	attempt := atomic.AddInt64(&c.stats.reconnectCount, 1)

	// 检查是否超过最大重连次数
	if c.config.ReconnectMaxCount > 0 && attempt > int64(c.config.ReconnectMaxCount) {
		logrus.Warnf("reached maximum reconnect attempts (%d), stopping reconnection", c.config.ReconnectMaxCount)
		c.emitEvent(Event{
			Type:      EventError,
			Error:     fmt.Errorf("maximum reconnect attempts reached: %d", c.config.ReconnectMaxCount),
			Timestamp: time.Now(),
		})
		return
	}

	delay := time.Duration(atomic.LoadInt64(&c.reconnectDelay))

	c.emitEvent(Event{
		Type:      EventReconnecting,
		Data:      attempt,
		Timestamp: time.Now(),
	})

	// 增强的日志显示重连进度
	if c.config.ReconnectMaxCount > 0 {
		logrus.Infof("attempting reconnection #%d/%d after %v delay",
			attempt, c.config.ReconnectMaxCount, delay)
	} else {
		logrus.Infof("attempting reconnection #%d (unlimited) after %v delay", attempt, delay)
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-c.ctx.Done():
		logrus.Info("reconnection cancelled due to context cancellation")
		return
	case <-timer.C:
		if err := c.Connect(); err != nil {
			// 指数退避
			newDelay := min(time.Duration(float64(delay)*c.config.ReconnectBackoff), c.config.MaxReconnectDelay)
			atomic.StoreInt64(&c.reconnectDelay, int64(newDelay))

			logrus.Warnf("reconnect attempt %d failed: %v, next attempt in %v",
				attempt, err, newDelay)

			// 检查是否还能继续重连
			if c.config.ReconnectMaxCount == 0 || attempt < int64(c.config.ReconnectMaxCount) {
				// 递归重连
				go c.reconnect()
			} else {
				logrus.Errorf("maximum reconnect attempts (%d) reached, giving up", c.config.ReconnectMaxCount)
				c.emitEvent(Event{
					Type:      EventError,
					Error:     fmt.Errorf("failed to reconnect after %d attempts", attempt),
					Timestamp: time.Now(),
				})
				c.Close()
			}
		} else {
			// 重连成功，重置延迟
			atomic.StoreInt64(&c.reconnectDelay, int64(c.config.ReconnectInterval))
			logrus.Infof("reconnection successful after %d attempts", attempt)
		}
	}
}

// 发送事件
func (c *WSClient) emitEvent(event Event) {
	select {
	case c.eventChan <- event:
	default:
		// 事件队列满，丢弃事件
		logrus.Warn("event queue is full, dropping event")
	}
}

// 重置重连计数（手动重置）
func (c *WSClient) ResetReconnectCount() {
	atomic.StoreInt64(&c.stats.reconnectCount, 0)
	atomic.StoreInt64(&c.reconnectDelay, int64(c.config.ReconnectInterval))
	logrus.Info("reconnect count and delay reset")
}
