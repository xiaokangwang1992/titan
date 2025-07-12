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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"plugin"
	"strings"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/piaobeizu/titan/config"
	"github.com/piaobeizu/titan/pkg/event"
	"github.com/sirupsen/logrus"
)

// type Plugin struct {
// 	ctx      context.Context
// 	cancel   context.CancelFunc
// 	Name     PluginName
// 	Version  string
// 	Location string
// 	Config   any
// 	queue    *event.Event
// 	pool     *ants.Pool
// }

// func (p *Plugin) Run() error {
// 	// do something
// 	plugin := p.create()
// 	if plugin == nil {
// 		return fmt.Errorf("plugin[%s] not found", p.Name)
// 	}
// 	if err := plugin.Init(); err != nil {
// 		return err
// 	}
// 	p.pool.Submit(plugin.Run)
// 	return nil
// }

// func (p *Plugin) Stop() {
// 	plugin := p.create()
// 	if plugin == nil {
// 		logrus.Errorf("plugin[%s] not found", p.Name)
// 	}
// 	p.cancel()
// 	plugin.Stop()
// }

// func (p *Plugin) Status() error {
// 	// do something
// 	return nil
// }

// func (p *Plugin) Health() PluginState {
// 	// do something
// 	plugin := p.create()
// 	if plugin == nil {
// 		logrus.Errorf("plugin[%s] not found", p.Name)
// 		return ""
// 	}
// 	return plugin.Health()
// }

// func (p *Plugin) create() PluginInterface {
// 	var plugin PluginInterface
// 	switch {
// 	}
// 	return plugin
// }

// func (p *Plugin) Restart() error {
// 	// do something
// 	return nil
// }

// func (p *Plugin) RefreshCongfig(cfg any) {
// 	// do something
// 	plugin := p.create()
// 	if plugin == nil {
// 		logrus.Errorf("plugin[%s] not found", p.Name)
// 		panic(fmt.Errorf("plugin[%s] not found", p.Name))
// 	}
// 	plugin.RefreshConfig(cfg)
// }

type PluginsConfig struct {
	Configs map[PluginName]any `yaml:"configs,omitempty"`
}

type Plugins struct {
	ctx           context.Context
	plugins       map[PluginName]Plugin
	config        *config.Plugin
	pluginsConfig PluginsConfig
	queue         *event.Event
	mu            sync.RWMutex
	pool          *ants.Pool
}

func NewPlugins(ctx context.Context, conf *config.Plugin, pool *ants.Pool) *Plugins {
	plugins := &Plugins{
		ctx:     ctx,
		plugins: map[PluginName]Plugin{},
		config:  conf,
		queue:   event.NewEvent(ctx, nil, nil),
		mu:      sync.RWMutex{},
		pluginsConfig: PluginsConfig{
			Configs: map[PluginName]any{},
		},
		pool: pool,
	}
	return plugins
}

func (p *Plugins) Start() {
	check := make(chan struct{}, 1)
	check <- struct{}{}
	timer := time.NewTimer(10 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			entries, err := os.ReadDir(p.config.Path)
			if err != nil {
				return
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				if !strings.HasSuffix(entry.Name(), ".so") {
					continue
				}
				plug, err := plugin.Open(entry.Name())
				if err != nil {
					logrus.Errorf("failed to load plugin: %+v", err)
					continue
				}
				newPlugin, err := plug.Lookup(fmt.Sprintf("New%sPlugin", entry.Name()))
				if err != nil {
					log.Printf("❌ symbol not found: %v", err)
					return
				}
				demoPlugin := newPlugin.(func() Plugin)()
				p.plugins[demoPlugin.GetName()] = demoPlugin
			}
			if p.config.Config != "" {
				var cfg PluginsConfig
				if err := json.Unmarshal([]byte(p.config.Config), &cfg); err == nil {
					p.pluginsConfig = cfg
				}
			}
			for _, plugin := range p.plugins {
				if plugin.Health() == PluginStateStopped {
					plugin.Stop()
					delete(p.plugins, plugin.GetName())
				}
				if plugin.Health() == PluginStateCreate {
					plugin.Run()
				}
				if plugin.Health() == PluginStateRunning {
					if config, ok := p.pluginsConfig.Configs[plugin.GetName()]; ok {
						plugin.RefreshConfig(config)
					}
				}
			}

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
	allDone := true
	for range int(p.config.GracefulShutdown) {
		time.Sleep(1 * time.Second)
		allDone = true
		for _, plugin := range p.plugins {
			if plugin.Health() != PluginStateStopped {
				allDone = false
				continue
			}
		}
		if allDone {
			break
		}
	}
	for _, plugin := range p.plugins {
		if plugin.Health() != PluginStateStopped {
			logrus.WithFields(logrus.Fields{"plugin": plugin.GetName()}).Warn("plugin can't stop")
		} else {
			logrus.WithFields(logrus.Fields{"plugin": plugin.GetName()}).Info("plugin stopped")
		}
	}
}
