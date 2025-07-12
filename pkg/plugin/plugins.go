/*
 * @Version : 1.0
 * @Author  : xiaokang.w
 * @Email   : xiaokang.w@gmicloud.ai
 * @Date    : 2025/07/12
 * @Desc    : 描述信息
 */

package plugin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/piaobeizu/titan/config"
	"github.com/piaobeizu/titan/pkg/event"
	"github.com/sirupsen/logrus"
)

type Plugin struct {
	ctx      context.Context
	cancel   context.CancelFunc
	Name     PluginName
	Version  string
	Location string
	Config   any
	queue    *event.Event
	pool     *ants.Pool
}

func (p *Plugin) Run() error {
	// do something
	plugin := p.create()
	if plugin == nil {
		return fmt.Errorf("plugin[%s] not found", p.Name)
	}
	if err := plugin.Init(); err != nil {
		return err
	}
	p.pool.Submit(plugin.Run)
	return nil
}

func (p *Plugin) Stop() {
	plugin := p.create()
	if plugin == nil {
		logrus.Errorf("plugin[%s] not found", p.Name)
	}
	p.cancel()
	plugin.Stop()
}

func (p *Plugin) Status() error {
	// do something
	return nil
}

func (p *Plugin) Health() PluginState {
	// do something
	plugin := p.create()
	if plugin == nil {
		logrus.Errorf("plugin[%s] not found", p.Name)
		return ""
	}
	return plugin.Health()
}

func (p *Plugin) create() PluginInterface {
	var plugin PluginInterface
	switch {
	}
	return plugin
}

func (p *Plugin) Restart() error {
	// do something
	return nil
}

func (p *Plugin) RefreshCongfig(cfg any) {
	// do something
	plugin := p.create()
	if plugin == nil {
		logrus.Errorf("plugin[%s] not found", p.Name)
		panic(fmt.Errorf("plugin[%s] not found", p.Name))
	}
	plugin.RefreshConfig(cfg)
}

type Plugins struct {
	ctx     context.Context
	plugins map[string]*Plugin
	config  *config.Plugin
	queue   *event.Event
	mu      sync.RWMutex
	pool    *ants.Pool
}

func NewPlugins(ctx context.Context, conf *config.Plugin, pool *ants.Pool) *Plugins {
	plugins := &Plugins{
		ctx:     ctx,
		plugins: map[string]*Plugin{},
		config:  conf,
		queue:   event.NewEvent(ctx, nil, nil),
		mu:      sync.RWMutex{},
		pool:    pool,
	}
	return plugins
}

func (p *Plugins) Start() {
	check := make(chan struct{}, 1)
	check <- struct{}{}
	// ticker for checking plugins
	timer := time.NewTimer(10 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			// ps := config.GetConfig().Plugin
			p.mu.Lock()

			// for _, plugin := range ps {
			// 	if !plugin.Enabled {
			// 		if _, ok := p.plugins[plugin.Name]; ok {
			// 			p.plugins[plugin.Name].Stop()
			// 			p.queue.Remove(plugin.Name)
			// 			delete(p.plugins, plugin.Name)
			// 		}
			// 	} else if _, ok := p.plugins[plugin.Name]; !ok {
			// 		p.queue.Register(plugin.Name)
			// 		newPlugin := NewPlugin(plugin, p.queue, p.pool)
			// 		p.plugins[plugin.Name] = newPlugin
			// 	} else if p.plugins[plugin.Name].Health() == plugins.PluginStateStopped ||
			// 		plugin.Version != p.plugins[plugin.Name].Version {
			// 		logrus.WithFields(logrus.Fields{"plugin": plugin.Name}).Warn("plugin is not running, restart it")
			// 		newPlugin := NewPlugin(plugin, p.queue, p.pool)
			// 		p.plugins[plugin.Name] = newPlugin
			// 	} else {
			// 		p.plugins[plugin.Name].RefreshCongfig(plugin.Config)
			// 	}
			// }
			// for _, plugin := range p.plugins {
			// 	if plugin.Health() == plugins.PluginStateCreate {
			// 		if err := plugin.Run(); err != nil {
			// 			logrus.WithFields(logrus.Fields{"plugin": plugin.Name}).Errorf("plugin run failed: %+v", err)
			// 		}
			// 	}
			// }
			p.mu.Unlock()
			logrus.Info("agent is running")
			timer.Reset(time.Duration(p.config.Refresh) * time.Second)
		case <-p.ctx.Done():
			close(check)
			return
		}
	}
}

func (p *Plugins) Stop() {
	for _, plugin := range p.plugins {
		p.pool.Submit(func() {
			plugin.Stop()
		})
	}
	// allDone := true
	// for range int(config.GetEnvs().GracefulShutdown) {
	// 	time.Sleep(1 * time.Second)
	// 	allDone = true
	// 	for _, plugin := range p.plugins {
	// 		if plugin.Health() != plugins.PluginStateStopped {
	// 			allDone = false
	// 			continue
	// 		}
	// 	}
	// 	if allDone {
	// 		break
	// 	}
	// }
	// for _, plugin := range p.plugins {
	// 	if plugin.Health() != plugins.PluginStateStopped {
	// 		logrus.WithFields(logrus.Fields{"plugin": plugin.Name}).Warn("plugin can't stop")
	// 	} else {
	// 		logrus.WithFields(logrus.Fields{"plugin": plugin.Name}).Info("plugin stopped")
	// 	}
	// }
}
