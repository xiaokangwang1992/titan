/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2024/01/21 11:07:45
 Desc     :
*/

package service

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/piaobeizu/titan/config"
	"github.com/sirupsen/logrus"
)

type ApiServer struct {
	ctx         context.Context
	srv         *http.Server
	routes      map[string]config.ApiGroup
	apiAddr     string
	version     string
	middlewares map[string]map[string]any
	handler     any
	wsHandler   any
	middleware  any
	stream      *event
	log         *logrus.Entry
	ws          *WebSocketServer
}

func NewApiServer(ctx context.Context, addr, version string, log *logrus.Entry) *ApiServer {
	return &ApiServer{
		ctx:         ctx,
		routes:      make(map[string]config.ApiGroup),
		apiAddr:     addr,
		middlewares: make(map[string]map[string]any),
		version:     version,
		log:         log,
	}
}

func (s *ApiServer) AddRoutes(group string, middlewares, routes, sses, websockets []string) {
	s.routes[group] = config.ApiGroup{
		Middlewares: middlewares,
		Routers:     routes,
		Sses:        sses,
		WebSockets:  websockets,
	}
}

func (s *ApiServer) AddMiddlewares(middlewares map[string]map[string]any) {
	s.middlewares = middlewares
}

func (s *ApiServer) AddHandler(handler any) {
	if reflect.ValueOf(handler).Kind() != reflect.Ptr {
		panic("handler must be a pointer")
	}
	s.handler = handler
}

func (s *ApiServer) AddWSHandler(handler any) {
	if reflect.ValueOf(handler).Kind() != reflect.Ptr {
		panic("handler must be a pointer")
	}
	s.wsHandler = handler
}

func (s *ApiServer) AddWSEvent(t ServerEventType, event ServerEventHandler) {
	if s.ws == nil {
		s.ws = NewWebSocketServer(s.ctx, s.apiAddr, s.version, s.log)
	}
	s.ws.OnEvent(t, event)
}

func (s *ApiServer) AddMiddleware(middleware any) {
	if reflect.ValueOf(middleware).Kind() != reflect.Ptr {
		panic("middleware must be a pointer")
	}
	s.middleware = middleware
}

func (s *ApiServer) Start() {
	s.log.Infof("start api service, listen on %s", s.apiAddr)

	r := gin.Default()
	rg := r.Group("/api")
	if s.version != "" {
		rg = rg.Group("/" + s.version)
	}
	s.bindRouter(rg)
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"version": "",
		})
	})
	// routers := r.Routes()
	// for _, v := range routers {
	// 	logrus.Debugf("[router] %-6s - %s", v.Method, v.Path)
	// }
	s.srv = &http.Server{
		Addr:    s.apiAddr,
		Handler: r,
	}
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Fatalf("Failed to start HTTP API service, because: %+v", err)
		}
	}()
}

func (s *ApiServer) Stop() {
	// Wait for 5 seconds to finish processing
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if s.ws != nil {
		s.ws.mu.Lock()
		for clientID := range s.ws.connections {
			s.ws.connections[clientID].Close()
			delete(s.ws.connections, clientID)
		}
		s.ws.mu.Unlock()
	}
	if err := s.srv.Shutdown(ctx); err != nil {
		s.log.Fatalf("Failed to shutdown HTTP API service, because: %+v", err)
	}
	<-ctx.Done()
	s.log.Info("HTTP API service shutdown.")
}

func (s *ApiServer) bindRouter(r *gin.RouterGroup) {
	for group, rs := range s.routes {
		if len(rs.Routers) > 0 {
			rg := r.Group("/" + group).Use(s.callMiddleware(rs.Middlewares, false)...)
			for _, r := range rs.Routers {
				rs := strings.Split(r, ",")
				if len(rs) != 3 {
					s.log.Fatalf("route config error, route: %s", r)
				}
				path := strings.TrimSpace(rs[0])
				method := strings.TrimSpace(rs[1])
				handler := strings.TrimSpace(rs[2])
				callHandler := s.callHandler(handler)
				if callHandler == nil {
					s.log.Fatalf("handler %s not found, route: %s", handler, r)
				}
				switch strings.ToUpper(method) {
				case "GET":
					rg.GET(path, callHandler)
				case "POST":
					rg.POST(path, callHandler)
				case "PUT":
					rg.PUT(path, callHandler)
				case "DELETE":
					rg.DELETE(path, callHandler)
				default:
					s.log.Fatalf("method %s not support, route: %s", method, r)
				}
			}
		}
		if len(rs.Sses) > 0 {
			s.stream = newEvent()
			rg := r.Group("/sse/" + group).Use(s.callMiddleware(rs.Middlewares, true)...)
			for _, sse := range rs.Sses {
				ss := strings.Split(sse, ",")
				if len(ss) != 3 {
					s.log.Fatalf("sse config error, route: %s", sse)
				}
				path := strings.TrimSpace(ss[0])
				method := strings.TrimSpace(ss[1])
				handler := strings.TrimSpace(ss[2])
				switch strings.ToUpper(method) {
				case "GET":
					rg.GET(path, s.callHandler(handler))
				case "POST":
					rg.POST(path, s.callHandler(handler))
				default:
					s.log.Fatalf("method %s not support, route: %s", method, sse)
				}
			}
		}
		if len(rs.WebSockets) > 0 {
			if s.ws == nil {
				s.ws = NewWebSocketServer(s.ctx, s.apiAddr, s.version, s.log)
			}
			go s.ws.handleBroadcast()
			go s.ws.eventLoop()

			rg := r.Group("/ws/" + group).Use(s.callMiddleware(rs.Middlewares, false)...)
			for _, ws := range rs.WebSockets {
				wss := strings.Split(ws, ",")
				if len(wss) != 2 {
					s.log.Fatalf("websocket config error, route: %s", ws)
				}
				path := strings.TrimSpace(wss[0])
				handler := strings.TrimSpace(wss[1])
				// WebSocket 只支持 GET 方法进行升级
				rg.GET(path, s.callWSHandler(handler))
				s.log.Infof("registered WebSocket route: /api/ws/%s/%s", group, path)
			}
		}
	}
}

func (s *ApiServer) callMiddleware(ms []string, sse bool) (mfs []gin.HandlerFunc) {
	if sse {
		mfs = append(mfs, sseHeadersMiddleware())
		if s.stream != nil {
			mfs = append(mfs, s.stream.serveHTTP())
		}
	}
	for _, m := range ms {
		if _, ok := s.middlewares[m]; !ok {
			s.log.Fatalf("middleware %s in route not found in http config.", m)
		}
		args := s.middlewares[m]
		m = strings.ToUpper(m[:1]) + m[1:] + "Middleware"
		if reflect.ValueOf(s.middleware).Elem().FieldByName("Args").IsValid() {
			reflect.ValueOf(s.middleware).Elem().FieldByName("Args").Set(reflect.ValueOf(args))
		}
		middleware := reflect.ValueOf(s.middleware).MethodByName(m).Interface()
		if sampleFunc, ok := middleware.(func(c *gin.Context)); ok {
			mfs = append(mfs, sampleFunc)
		} else {
			s.log.Fatalf("Conversion middlwware %s failed.", m)
		}
	}
	return
}

func (s *ApiServer) callHandler(f string) gin.HandlerFunc {
	handler := reflect.ValueOf(s.handler).MethodByName(f).Interface()
	if sampleFunc, ok := handler.(func(c *gin.Context)); ok {
		return sampleFunc
	} else {
		s.log.Fatalf("Conversion handler %s failed, handler: %s", f, reflect.TypeOf(handler).String())
	}
	return nil
}

// createWSHandler 创建 WebSocket 处理器
func (s *ApiServer) callWSHandler(f string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取客户端ID
		clientID := c.Query("client_id")
		if clientID == "" {
			s.log.Errorf("missing client_id parameter")
			return
		}

		// 升级到 WebSocket 连接
		conn, err := s.ws.upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			s.log.Errorf("websocket upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		conn.SetPingHandler(func(message string) error {
			return conn.WriteMessage(websocket.PongMessage, []byte("pong"))
		})

		conn.SetPongHandler(func(message string) error {
			return nil
		})

		// 注册连接
		s.ws.addConnection(clientID, conn)
		s.log.Infof("websocket connection established: %s", clientID)

		// 清理连接
		defer func() {
			s.ws.removeConnection(clientID)
			s.log.Infof("websocket connection closed: %s", clientID)
		}()

		// 发送欢迎消息
		s.ws.SendText(clientID, "Connected to WebSocket server")

		// 获取处理器方法
		handler := reflect.ValueOf(s.wsHandler).MethodByName(f).Interface()
		if wsHandler, ok := handler.(func(*gin.Context, []byte) (any, error)); ok {
			// 消息处理循环
			for {
				select {
				case <-s.ctx.Done():
					s.log.Infof("websocket connection closed: %s", clientID)
					return
				default:
					t, data, err := conn.ReadMessage()
					if err != nil {
						s.log.Errorf("websocket read message failed: %v", err)
						if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
							s.log.Errorf("websocket read message failed: %v", err)
						}
						return
					}

					// 使用预处理器过滤消息
					if !s.ws.preprocessMessage(data, clientID) {
						continue // 跳过不需要处理的消息
					}
					// 根据消息类型处理
					switch t {
					case websocket.TextMessage, websocket.BinaryMessage:
						// 调用处理器
						resp, err := wsHandler(c, data)
						if err != nil {
							s.log.Errorf("websocket handler error: %v", err)
							conn.WriteJSON(WSMessage[any]{
								Type:      WSMSG_TYPE_ERROR,
								Timestamp: time.Now().Unix(),
								ClientID:  clientID,
								Data:      err,
							})
							continue
						}
						if resp != nil {
							switch v := resp.(type) {
							case WSMessage[any], *WSMessage[any]:
								msg := v.(WSMessage[any])
								msg.ClientID = clientID
								msg.Timestamp = time.Now().Unix()
								if err := conn.WriteJSON(msg); err != nil {
									s.log.Errorf("failed to send response to %s: %v", clientID, err)
								}
							case string, *string:
								if err := conn.WriteMessage(websocket.TextMessage, []byte(v.(string))); err != nil {
									s.log.Errorf("failed to send response to %s: %v", clientID, err)
								}
							case []byte, *[]byte:
								if err := conn.WriteMessage(websocket.BinaryMessage, v.([]byte)); err != nil {
									s.log.Errorf("failed to send response to %s: %v", clientID, err)
								}
							default:
								if err := conn.WriteJSON(resp); err != nil {
									s.log.Errorf("failed to send response to %s: %v", clientID, err)
								}
							}
						} else {
							s.log.Debugf("websocket handler response is nil: %s", clientID)
						}
					default:
						s.log.Debugf("unsupported message type %d from %s", t, clientID)
					}
				}
			}
		} else {
			s.log.Errorf("WebSocket handler %s conversion failed", f)
		}
	}
}

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type ApiHandler struct {
	Response Response
}

type ApiMiddleware struct {
	Args map[string]any
}

func (m *ApiMiddleware) DefaultMiddleware(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Next()
}
