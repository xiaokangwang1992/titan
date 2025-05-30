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
	"github.com/piaobeizu/titan/types"
	"github.com/sirupsen/logrus"
)

type ApiServer struct {
	ctx         context.Context
	srv         *http.Server
	routes      map[string]types.ApiGroup
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
		routes:      make(map[string]types.ApiGroup),
		apiAddr:     addr,
		middlewares: make(map[string]map[string]any),
		version:     version,
		log:         log,
	}
}

func (s *ApiServer) AddRoutes(group string, middlewares, routes, sses, websockets []string) {
	s.routes[group] = types.ApiGroup{
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
					panic(r + " route config error")
				}
				path := strings.TrimSpace(rs[0])
				method := strings.TrimSpace(rs[1])
				handler := strings.TrimSpace(rs[2])
				switch strings.ToUpper(method) {
				case "GET":
					rg.GET(path, s.callHandler(handler))
				case "POST":
					rg.POST(path, s.callHandler(handler))
				case "PUT":
					rg.PUT(path, s.callHandler(handler))
				case "DELETE":
					rg.DELETE(path, s.callHandler(handler))
				default:
					panic(r + " method not support")
				}
			}
		}
		if len(rs.Sses) > 0 {
			s.stream = newEvent()
			rg := r.Group("/sse/" + group).Use(s.callMiddleware(rs.Middlewares, true)...)
			for _, sse := range rs.Sses {
				ss := strings.Split(sse, ",")
				if len(ss) != 3 {
					panic(sse + " sse config error")
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
					panic(sse + " method not support")
				}
			}
		}
		if len(rs.WebSockets) > 0 {
			s.ws = NewWebSocketServer(s.ctx, s.apiAddr, s.version, s.log)
			go s.ws.handleBroadcast()

			rg := r.Group("/ws/" + group).Use(s.callMiddleware(rs.Middlewares, false)...)
			for _, ws := range rs.WebSockets {
				wss := strings.Split(ws, ",")
				if len(wss) != 2 {
					panic(ws + " websocket config error, format: path,handler")
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
			panic("middleware in route not found in http config.")
		}
		args := s.middlewares[m]
		m = strings.ToUpper(m[:1]) + m[1:] + "Middleware"
		if !reflect.ValueOf(s.middleware).Elem().FieldByName("Args").IsValid() {
			reflect.ValueOf(s.middleware).Elem().FieldByName("Args").Set(reflect.ValueOf(args))
		}
		middleware := reflect.ValueOf(s.middleware).MethodByName(m).Interface()
		if sampleFunc, ok := middleware.(func(c *gin.Context)); ok {
			mfs = append(mfs, sampleFunc)
		} else {
			panic("Conversion middlwware failed.")
		}
	}
	return
}

func (s *ApiServer) callHandler(f string) gin.HandlerFunc {
	handler := reflect.ValueOf(s.handler).MethodByName(f).Interface()
	// 使用类型断言获取具体的函数
	if sampleFunc, ok := handler.(func(c *gin.Context)); ok {
		// logrus.Debugf("[router] %-6s - %-16s %s", m, p, f)
		// 调用具体的函数
		return sampleFunc
	} else {
		panic("Conversion handler failed.")
	}
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

		// 注册连接
		s.ws.mu.Lock()
		s.ws.connections[clientID] = conn
		s.ws.mu.Unlock()
		s.log.Infof("websocket connection established: %s", clientID)

		// 清理连接
		defer func() {
			s.ws.mu.Lock()
			delete(s.ws.connections, clientID)
			s.ws.mu.Unlock()
			s.log.Infof("websocket connection closed: %s", clientID)
		}()

		// 发送欢迎消息
		welcomeMsg := WSMessage{
			Type:      WSMSG_TYPE_WELCOME,
			Data:      "Connected to WebSocket server",
			Timestamp: time.Now().Unix(),
			ClientID:  clientID,
		}
		s.ws.Send(clientID, welcomeMsg)

		// 获取处理器方法
		handler := reflect.ValueOf(s.wsHandler).MethodByName(f).Interface()
		if wsHandler, ok := handler.(func(*gin.Context, *websocket.Conn, *WSMessage)); ok {
			// 消息处理循环
			for {
				select {
				case <-s.ctx.Done():
					s.log.Infof("websocket connection closed: %s", clientID)
					return
				default:
					var msg WSMessage
					if err := conn.ReadJSON(&msg); err != nil {
						s.log.Errorf("websocket read message failed: %v", err)
						if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
							s.log.Errorf("websocket read message failed: %v", err)
						}
						return
					}
					if msg.Type == WSMSG_TYPE_PING {
						s.log.Debugf("websocket ping message received: %s", clientID)
						continue
					}

					// 设置消息元数据
					msg.ClientID = clientID
					msg.Timestamp = time.Now().Unix()

					// 调用处理器
					wsHandler(c, conn, &msg)
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
