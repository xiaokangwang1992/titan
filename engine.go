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

	"github.com/piaobeizu/titan/service"
	"github.com/sirupsen/logrus"
)

type Titan struct {
	app       string
	ctx       context.Context
	cancel    context.CancelFunc
	api       *service.ApiServer
	scheduler *service.Scheduler
}

func NewTitan(ctx context.Context, cancel context.CancelFunc, app string) *Titan {
	logrus.Infof("welcome to app %s", app)
	e := &Titan{
		app:    app,
		ctx:    ctx,
		cancel: cancel,
	}
	return e
}

func (t *Titan) ApiServer(addr string) *Titan {
	if addr != "" {
		apiCtx := context.WithoutCancel(t.ctx)
		t.api = service.NewApiServer(apiCtx, addr)
	}
	return t
}

func (t *Titan) Routers(group string, middlewares []string, routes []string) *Titan {
	if t.api == nil {
		panic("api is nil")
	}
	t.api.AddRoutes(group, middlewares, routes)
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
	t.scheduler.Add(job)
	return t
}

func (e *Titan) Start() {
	if e.api != nil {
		e.api.Start()
	}
	if e.scheduler != nil {
		e.scheduler.Start()
	}
	<-e.ctx.Done()
}

func (e *Titan) Stop() {
	e.cancel()
	e.api.Stop()
	e.scheduler.Stop()
	logrus.Printf("titan stopped, byebye!")
}
