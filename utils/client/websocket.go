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
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/piaobeizu/titan/service"
	"github.com/piaobeizu/titan/utils"
	"github.com/sirupsen/logrus"
)

type con struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc
	url    string
	conn   *websocket.Conn
	send   chan service.WSMessage
	status int
}

func (c *con) connect() error {
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("parse url failed: %v", err)
	}

	logrus.Infof("connecting to %s", c.url)

	// set headers
	headers := make(map[string][]string)
	headers["User-Agent"] = []string{"Go-WebSocket-Client/1.0"}

	// connect to websocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), headers)
	if err != nil {
		return fmt.Errorf("connect failed: %v", err)
	}

	c.conn = conn
	logrus.Info("websocket connect success")

	return nil
}

func (c *con) read() {
	defer func() {
		c.conn.Close()
		c.status = 0
	}()

	// set read timeout
	wsReadTimeout := utils.GetEnv("WS_READ_TIMEOUT", "1s")
	wsReadTimeoutDuration, err := time.ParseDuration(wsReadTimeout)
	if err != nil {
		logrus.Errorf("parse ws read timeout failed: %v", err)
		return
	}
	c.conn.SetReadDeadline(time.Now().Add(wsReadTimeoutDuration))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(wsReadTimeoutDuration))
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			_, messageBytes, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logrus.Errorf("websocket unexpected close: %v", err)
				}
				return
			}

			var msg service.WSMessage
			if err := json.Unmarshal(messageBytes, &msg); err != nil {
				logrus.Errorf("parse message failed: %v, original message: %s", err, string(messageBytes))
				continue
			}
		}
	}
}

// writePump handle send message to server
func (c *con) write() {
	ticker := time.NewTicker(1 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		c.status = 0
	}()

	wsWriteTimeout := utils.GetEnv("WS_WRITE_TIMEOUT", "1s")
	wsWriteTimeoutDuration, err := time.ParseDuration(wsWriteTimeout)
	if err != nil {
		logrus.Errorf("parse ws write timeout failed: %v", err)
		return
	}
	// set write deadline
	c.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeoutDuration))

	for {
		select {
		case <-c.ctx.Done():
			return
		case message := <-c.send:
			// serialize message
			messageBytes, err := json.Marshal(message)
			if err != nil {
				logrus.Errorf("serialize message failed: %v", err)
				continue
			}

			// send message
			if err := c.conn.WriteMessage(websocket.TextMessage, messageBytes); err != nil {
				logrus.Errorf("send message failed: %v", err)
				return
			}
			logrus.Infof("send message: %s", message.Type)
		case <-ticker.C:
			// set write deadline
			c.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeoutDuration))
			// send ping message
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type WSClient struct {
	id     string
	con    *con
	url    string
	send   chan service.WSMessage
	ctx    context.Context
	cancel context.CancelFunc
}

func NewWSClient(ctx context.Context, id, url string) *WSClient {
	return &WSClient{
		ctx:  ctx,
		id:   id,
		url:  url,
		send: make(chan service.WSMessage, 1),
	}
}

// Start 启动客户端，开始监听消息
func (c *WSClient) Start() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			if c.con != nil {
				c.con.conn.SetWriteDeadline(time.Now().Add(time.Second))
				c.con.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			}
			return
		case msg := <-c.send:
			if c.con != nil {
				c.con.send <- msg
			}
		case <-ticker.C:
			if c.con != nil && c.con.status == 0 {
				c.cancel()
				c.con = nil
			}
			subctx, subcancel := context.WithCancel(c.ctx)
			c.con = &con{
				id:     c.id,
				ctx:    subctx,
				url:    c.url,
				send:   c.send,
				cancel: subcancel,
				status: 1,
			}
			if err := c.con.connect(); err != nil {
				logrus.Errorf("websocket connection failed: %v", err)
				return
			}
			// start read
			go c.con.read()
			// start write
			go c.con.write()
		}
	}
}

func (c *WSClient) Send(msg service.WSMessage) {
	c.send <- msg
}
