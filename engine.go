/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2024/08/20 15:43:10
 Desc     :
*/

package titan

import (
	"context"
	"os"
	"os/signal"
	"reflect"
	"syscall"

	"github.com/piaobeizu/titan/log"
	"github.com/piaobeizu/titan/service"
	"github.com/sirupsen/logrus"
)

type Titan struct {
	app       string
	ctx       context.Context
	api       *service.ApiServer
	scheduler *service.Scheduler
	singal    chan os.Signal
	log       *logrus.Entry
}

func NewTitan(ctx context.Context, app, logMode string) *Titan {
	log.InitLog(app, logMode)
	e := &Titan{
		app:    app,
		ctx:    ctx,
		singal: make(chan os.Signal, 1),
		log:    logrus.WithField("app", app),
	}
	signal.Notify(e.singal, syscall.SIGTERM)
	signal.Notify(e.singal, syscall.SIGINT)
	signal.Notify(e.singal, syscall.SIGQUIT)
	signal.Notify(e.singal, syscall.SIGHUP)
	e.log.Infof("welcome to app %s", app)
	return e
}

func (t *Titan) ApiServer(addr, version string) *Titan {
	if addr != "" {
		apiCtx := context.WithoutCancel(t.ctx)
		t.api = service.NewApiServer(apiCtx, addr, version, t.log)
	}
	return t
}

func (t *Titan) Routers(group string, middlewares, routes, sses, websockets []string) *Titan {
	if t.api == nil {
		panic("api is nil")
	}
	t.api.AddRoutes(group, middlewares, routes, sses, websockets)
	return t
}

func (t *Titan) Handler(handler any) *Titan {
	if t.api == nil {
		panic("api is nil")
	}
	t.api.AddHandler(handler)
	return t
}

func (t *Titan) WSHandler(handler any) *Titan {
	if t.api == nil {
		panic("api is nil")
	}
	t.api.AddWSHandler(handler)
	return t
}

func (t *Titan) OnWSEvent(eventType service.ServerEventType, event service.ServerEventHandler) *Titan {
	if t.api == nil {
		panic("api is nil")
	}
	t.api.AddWSEvent(eventType, event)
	return t
}

func (t *Titan) Middleware(middleware any) *Titan {
	if t.api == nil {
		panic("api is nil")
	}
	t.api.AddMiddleware(middleware)
	return t
}

func (t *Titan) Middlewares(middlewares map[string]map[string]any) *Titan {
	if t.api == nil {
		panic("api is nil")
	}
	t.api.AddMiddlewares(middlewares)
	return t
}

func (t *Titan) Scheduler() *Titan {
	schedCtx := context.WithoutCancel(t.ctx)
	t.scheduler = service.NewScheduler(schedCtx, t.log)
	return t
}

func (t *Titan) Job(job *service.Job) *Titan {
	if t.scheduler == nil {
		panic("scheduler is nil")
	}
	if reflect.ValueOf(job).Kind() != reflect.Ptr {
		panic("job must be a pointer")
	}
	t.scheduler.AddJob(job)
	return t
}

func (e *Titan) Start() {
	if e.api != nil {
		e.api.Start()
	}
	if e.scheduler != nil {
		e.scheduler.Start()
	}
	select {
	case <-e.ctx.Done():
		e.log.Info("context done, stop the titan")
		return
	case <-e.singal:
		e.log.Info("receive signal, stop the titan")
		return
	}
}

func (e *Titan) Stop() {
	if e.scheduler != nil {
		e.scheduler.Stop()
	}
	if e.api != nil {
		e.api.Stop()
	}
	e.log.Info("titan stopped, byebye!")
}
