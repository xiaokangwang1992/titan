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
	"log"
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/piaobeizu/titan/config"
	"github.com/piaobeizu/titan/pkg/event"
	"github.com/piaobeizu/titan/pkg/utils"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type PluginsConfig struct {
	Plugins map[PluginName]any `yaml:"plugins,omitempty"`
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
			Plugins: map[PluginName]any{},
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
				plug, err := plugin.Open(filepath.Join(p.config.Path, entry.Name()))
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
				if conf, err := utils.ReadFileContent(p.config.Config); err == nil {
					if err := yaml.Unmarshal([]byte(conf), &cfg); err == nil {
						p.pluginsConfig.Plugins = cfg.Plugins
					}
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
					if config, ok := p.pluginsConfig.Plugins[plugin.GetName()]; ok {
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
