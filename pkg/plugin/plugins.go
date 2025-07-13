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
	"io"
	"log"
	"os"
	"plugin"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/piaobeizu/titan/config"
	"github.com/piaobeizu/titan/pkg/event"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v2"
)

type PluginConfig struct {
	Path    string `yaml:"path,omitempty"`
	Version string `yaml:"version,omitempty"`
	Name    string `yaml:"name,omitempty"`
	Config  any    `yaml:"config,omitempty"`
}

type PluginsConfig struct {
	Plugins map[PluginName]PluginConfig `yaml:"plugins,omitempty"`
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
			Plugins: map[PluginName]PluginConfig{},
		},
		pool: pool,
	}
	return plugins
}

func (p *Plugins) Start() {
	check := make(chan struct{}, 1)
	check <- struct{}{}
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			p.mu.Lock()
			if p.config.Config != "" {
				var cfg PluginsConfig
				if conf, err := readPluginConfig(p.config.Config); err == nil {
					if err := yaml.Unmarshal(conf, &cfg); err == nil {
						p.pluginsConfig.Plugins = cfg.Plugins
					}
				}
			}
			for name, ps := range p.pluginsConfig.Plugins {
				if _, ok := p.plugins[name]; ok {
					continue
				}
				plug, err := plugin.Open(ps.Path)
				if err != nil {
					logrus.Errorf("failed to load plugin: %+v", err)
					continue
				}
				pluginName := cases.Title(language.Und).String(string(name))
				newPlugin, err := plug.Lookup(fmt.Sprintf("New%sPlugin", pluginName))
				if err != nil {
					log.Printf("❌ symbol not found: %v", err)
					continue
				}
				np := newPlugin.(func(context.Context, PluginName, any) Plugin)(p.ctx, name, ps.Config)
				p.plugins[name] = np
			}
			p.mu.Unlock()
			for _, plugin := range p.plugins {
				if plugin.Health() == PluginStateStopped {
					plugin.Stop()
					delete(p.plugins, plugin.GetName())
				}
				if plugin.Health() == PluginStateCreate {
					p.pool.Submit(func() {
						plugin.Run()
					})
				}
				if plugin.Health() == PluginStateRunning {
					if config, ok := p.pluginsConfig.Plugins[plugin.GetName()]; ok {
						plugin.RefreshConfig(config.Config)
					}
				}
			}

			logrus.Infof("all %d plugins are running", len(p.plugins))
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

func readPluginConfig(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	return content, err
}
