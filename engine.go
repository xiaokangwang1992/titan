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
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/GMISWE/ieops-plugins/event"
	iplugin "github.com/GMISWE/ieops-plugins/plugin"
	"github.com/panjf2000/ants/v2"
	"github.com/piaobeizu/titan/config"
	"github.com/piaobeizu/titan/pkg/cron"
	"github.com/piaobeizu/titan/pkg/log"
	"github.com/piaobeizu/titan/pkg/plugin"
	"github.com/piaobeizu/titan/pkg/service"
	"github.com/piaobeizu/titan/pkg/utils"
	"github.com/sirupsen/logrus"
)

type Titan struct {
	app       string
	ctx       context.Context
	api       *service.ApiServer
	scheduler *cron.Scheduler
	event     *event.Event
	pool      *ants.Pool
	plugin    *plugin.Plugin
	stop      chan struct{}
	singal    chan os.Signal
	log       *logrus.Entry
}

func (t *Titan) handlepanic(i any) {
	file, line := utils.FindCaller(5)
	logrus.WithFields(logrus.Fields{
		"panic": fmt.Sprintf("%s:%d", file, line),
	}).Errorf("%v\n%s", i, string(debug.Stack()))
	t.Stop()
}

func NewTitan(ctx context.Context, app, logMode string) *Titan {
	log.InitLog(app, logMode)
	t := &Titan{
		app:    app,
		ctx:    ctx,
		singal: make(chan os.Signal, 1),
		log:    logrus.WithField("app", app),
	}
	// 安全地获取Ants配置，如果为nil则使用默认值
	antsConfig := config.GetConfig().Ants
	poolSize := 100 // 默认值
	if antsConfig != nil {
		poolSize = antsConfig.Size
	}

	pool, err := ants.NewPool(poolSize, func(opts *ants.Options) {
		opts.ExpiryDuration = 60 * time.Second
		opts.Nonblocking = false
		opts.PreAlloc = true
		opts.MaxBlockingTasks = 10
		opts.PanicHandler = t.handlepanic
	})
	if err != nil {
		panic(err)
	}
	t.pool = pool
	// 安全地获取Event配置，如果为nil则使用默认值
	eventConfig := config.GetConfig().Event
	msgSize := 1000 // 默认值
	if eventConfig != nil {
		msgSize = eventConfig.MsgSize
	}

	t.event = event.NewEvent(t.ctx, []event.EventOption{
		event.WithConfig(&event.Config{
			MsgSize: msgSize,
		}),
		event.WithPool(t.pool)}...)
	signal.Notify(t.singal, syscall.SIGTERM)
	signal.Notify(t.singal, syscall.SIGINT)
	signal.Notify(t.singal, syscall.SIGQUIT)
	signal.Notify(t.singal, syscall.SIGHUP)
	t.log.Infof("welcome to app %s", app)
	return t
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
	t.scheduler = cron.NewScheduler(schedCtx, t.log)
	return t
}

func (t *Titan) Job(job *cron.Job) *Titan {
	if t.scheduler == nil {
		panic("scheduler is nil")
	}
	if reflect.ValueOf(job).Kind() != reflect.Ptr {
		panic("job must be a pointer")
	}
	t.scheduler.AddJob(job)
	return t
}

func (t *Titan) Plugins(conf *config.Plugin) *Titan {
	if conf == nil {
		conf = &config.Plugin{
			Refresh:          1,
			GracefulShutdown: 0,
		}
	}
	if conf.GracefulShutdown == 0 {
		conf.GracefulShutdown = 3
	}
	if t.plugin == nil {
		t.plugin = plugin.NewPlugin(
			t.ctx,
			plugin.WithRefresh(conf.Refresh),
			plugin.WithRedisKey(conf.RedisKey),
			plugin.WithPool(t.pool),
			plugin.WithEvent(t.event),
			plugin.WithConfig(&iplugin.PluginManagerConfig{
				GracefulShutdown: conf.GracefulShutdown,
				EventSize:        config.GetConfig().Event.MsgSize,
				PoolSize:         config.GetConfig().Ants.Size,
				Md5Check:         conf.Md5Check,
			}),
		)
	}
	return t
}

func (e *Titan) Start() {
	if e.api != nil {
		e.api.Start()
	}
	if e.scheduler != nil {
		e.scheduler.Start()
	}
	if e.plugin != nil {
		e.plugin.Start()
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
	if e.plugin != nil {
		e.plugin.Stop()
	}
	e.log.Info("titan stopped, byebye!")
}
