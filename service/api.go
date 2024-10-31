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
	middleware  any
	stream      *event
}

func NewApiServer(ctx context.Context, addr, version string) *ApiServer {
	return &ApiServer{
		ctx:         ctx,
		routes:      make(map[string]types.ApiGroup),
		apiAddr:     addr,
		middlewares: make(map[string]map[string]any),
		version:     version,
	}
}

func (s *ApiServer) AddRoutes(group string, middlewares []string, routes []string, sses []string) {
	s.routes[group] = types.ApiGroup{
		Middlewares: middlewares,
		Routers:     routes,
		Sses:        sses,
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

func (s *ApiServer) AddMiddleware(middleware any) {
	if reflect.ValueOf(middleware).Kind() != reflect.Ptr {
		panic("middleware must be a pointer")
	}
	s.middleware = middleware
}

func (s *ApiServer) Start() {
	logrus.Infof("start api service, listen on %s", s.apiAddr)
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
			logrus.Fatalf("Failed to start HTTP API service, because: %+v", err)
		}
	}()
}

func (s *ApiServer) Stop() {
	// Wait for 5 seconds to finish processing
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(ctx); err != nil {
		logrus.Fatalf("Failed to shutdown HTTP API service, because: %+v", err)
	}
	<-ctx.Done()
	logrus.Info("HTTP API service shutdown.")
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
