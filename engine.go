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

	"github.com/piaobeizu/titan/service"
	"github.com/sirupsen/logrus"
)

type Titan struct {
	app       string
	ctx       context.Context
	api       *service.ApiServer
	scheduler *service.Scheduler
	singal    chan os.Signal
}

func NewTitan(ctx context.Context, app string) *Titan {
	logrus.Infof("welcome to app %s", app)
	e := &Titan{
		app:    app,
		ctx:    ctx,
		singal: make(chan os.Signal, 1),
	}
	signal.Notify(e.singal, syscall.SIGTERM)
	signal.Notify(e.singal, syscall.SIGINT)
	signal.Notify(e.singal, syscall.SIGQUIT)
	signal.Notify(e.singal, syscall.SIGHUP)
	return e
}

func (t *Titan) ApiServer(addr, version string) *Titan {
	if addr != "" {
		apiCtx := context.WithoutCancel(t.ctx)
		t.api = service.NewApiServer(apiCtx, addr, version)
	}
	return t
}

func (t *Titan) Routers(group string, middlewares []string, routes []string, sses []string) *Titan {
	if t.api == nil {
		panic("api is nil")
	}
	t.api.AddRoutes(group, middlewares, routes, sses)
	return t
}

func (t *Titan) Handler(handler any) *Titan {
	if t.api == nil {
		panic("api is nil")
	}
	t.api.AddHandler(handler)
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
	t.scheduler = service.NewScheduler(schedCtx)
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
		logrus.Info("context done, stop the titan")
		return
	case <-e.singal:
		logrus.Info("receive signal, stop the titan")
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
	logrus.Printf("titan stopped, byebye!")
}
