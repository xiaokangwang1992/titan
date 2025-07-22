/*
 * @Version : 1.0
 * @Author  : xiaokang.w
 * @Email   : xiaokang.w@gmicloud.ai
 * @Date    : 2025/07/18
 * @Desc    : 描述信息
 */

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/GMISWE/ieops-plugins/event"
	"github.com/GMISWE/ieops-plugins/plugin"
	"github.com/panjf2000/ants/v2"
	"github.com/piaobeizu/titan/pkg/storage"
	"github.com/sirupsen/logrus"
)

type PluginRuntime struct {
	Path        string
	Version     string
	Enabled     bool
	CreateAt    string
	Symbol      string
	Description string
	Config      any
	State       string
}

type PluginOption func(*Plugin)

func WithRefresh(refresh int) PluginOption {
	return func(p *Plugin) {
		p.refresh = refresh
	}
}

func WithRedisKey(redisKey string) PluginOption {
	return func(p *Plugin) {
		p.redisBaseKey = redisKey
	}
}

func WithPool(pool *ants.Pool) PluginOption {
	return func(p *Plugin) {
		p.pool = pool
	}
}

func WithEvent(event *event.Event) PluginOption {
	return func(p *Plugin) {
		p.event = event
	}
}

func WithConfig(conf *plugin.PluginManagerConfig) PluginOption {
	return func(p *Plugin) {
		p.conf = conf
	}
}

func loadOptions(opts ...PluginOption) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

type Plugin struct {
	ctx          context.Context
	mu           sync.RWMutex
	refresh      int
	redisBaseKey string
	pm           *plugin.PluginManager
	pool         *ants.Pool
	event        *event.Event
	conf         *plugin.PluginManagerConfig
}

var (
	pluginsKey = "plugins"
)

func NewPlugin(ctx context.Context, opts ...PluginOption) *Plugin {
	p := loadOptions(opts...)
	if p.refresh == 0 {
		p.refresh = 10
	}
	if p.redisBaseKey == "" {
		p.redisBaseKey = "plugins"
	}

	return &Plugin{
		ctx:          ctx,
		mu:           sync.RWMutex{},
		refresh:      p.refresh,
		redisBaseKey: p.redisBaseKey,
		pm: plugin.NewPluginManager(
			ctx, plugin.WithPool(p.pool),
			plugin.WithEvent(p.event),
			plugin.WithConfig(p.conf),
		),
	}
}

func (p *Plugin) Start() {
	pm := plugin.NewPluginManager(p.ctx)
	ticker := time.NewTicker(time.Second * time.Duration(p.refresh))
	defer ticker.Stop()
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			pluginsCfgs, err := p.getPlugins(pluginsKey)
			if err != nil {
				logrus.Errorf("failed to get plugins: %+v", err)
				continue
			}
			for name, plugins := range pluginsCfgs {
				for _, plug := range plugins {
					if plug.Enabled {
						if err := pm.StartPlugin(&plugin.PluginDefinition{
							PluginName:  name,
							PluginPath:  plug.Path,
							Version:     plug.Version,
							ModifyAt:    plug.CreateAt,
							SymbolName:  plug.Symbol,
							Description: plug.Description,
							Config:      plug.Config,
						}); err != nil {
							logrus.Errorf("failed to start plugin: %+v", err)
							continue
						}
					} else {
						if err := pm.StopPlugin(name, plug.Version); err != nil {
							logrus.Errorf("failed to stop plugin: %+v", err)
							continue
						}
					}
				}
			}
		}
	}
}

func (p *Plugin) AddPlugin(define *plugin.PluginDefinition) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	pluginsCfgs, err := p.getPlugins(pluginsKey)
	if err != nil {
		logrus.Errorf("failed to get plugins: %+v", err)
		return err
	}
	pluginsCfgs[define.PluginName] = append(pluginsCfgs[define.PluginName], plugin.PluginConfig{
		Path:        define.PluginPath,
		Version:     define.Version,
		Enabled:     true,
		CreateAt:    define.ModifyAt,
		Symbol:      define.SymbolName,
		Description: define.Description,
		Config:      define.Config,
	})
	data, err := json.Marshal(pluginsCfgs)
	if err != nil {
		logrus.Errorf("failed to marshal plugins: %+v", err)
		return err
	}
	storage.RedisClient().Set(fmt.Sprintf("%s:%s", p.redisBaseKey, pluginsKey), string(data), 0)
	return nil
}

func (p *Plugin) StopPlugin(name plugin.PluginName, version string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	pluginsCfgs, err := p.getPlugins(pluginsKey)
	if err != nil {
		logrus.Errorf("failed to get plugins: %+v", err)
		return err
	}
	for i, plug := range pluginsCfgs[name] {
		if plug.Version == version {
			pluginsCfgs[name][i] = plugin.PluginConfig{
				Path:        plug.Path,
				Version:     plug.Version,
				Enabled:     false,
				CreateAt:    plug.CreateAt,
				Symbol:      plug.Symbol,
				Description: plug.Description,
				Config:      plug.Config,
			}
			break
		}
	}
	data, err := json.Marshal(pluginsCfgs)
	if err != nil {
		logrus.Errorf("failed to marshal plugins: %+v", err)
		return err
	}
	storage.RedisClient().Set(fmt.Sprintf("%s:%s", p.redisBaseKey, pluginsKey), string(data), 0)
	return nil
}

func (p *Plugin) DeletePlugin(name plugin.PluginName, version string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	pluginsCfgs, err := p.getPlugins(pluginsKey)
	if err != nil {
		logrus.Errorf("failed to get plugins: %+v", err)
		return err
	}
	for i, plug := range pluginsCfgs[name] {
		if plug.Version == version {
			pluginsCfgs[name] = append(pluginsCfgs[name][:i], pluginsCfgs[name][i+1:]...)
			break
		}
	}
	data, err := json.Marshal(pluginsCfgs)
	if err != nil {
		logrus.Errorf("failed to marshal plugins: %+v", err)
		return err
	}
	if err = p.pm.DeletePlugin(name, version); err != nil {
		logrus.Errorf("failed to delete plugin: %+v", err)
		return err
	}
	storage.RedisClient().Set(fmt.Sprintf("%s:%s", p.redisBaseKey, pluginsKey), string(data), 0)
	return nil
}

func (p *Plugin) ListPlugins() (map[plugin.PluginName][]PluginRuntime, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	pluginCfgs, err := p.getPlugins(pluginsKey)
	if err != nil {
		logrus.Errorf("failed to get plugins: %+v", err)
		return nil, err
	}
	allRuntimes := p.pm.GetAllRuntimes()
	pluginRuntimes := make(map[plugin.PluginName][]PluginRuntime)
	for name, plugins := range pluginCfgs {
		for _, plug := range plugins {
			pluginRuntimes[name] = append(pluginRuntimes[name], PluginRuntime{
				Path:        plug.Path,
				Version:     plug.Version,
				Enabled:     plug.Enabled,
				CreateAt:    plug.CreateAt,
				Symbol:      plug.Symbol,
				Description: plug.Description,
				Config:      plug.Config,
				State:       allRuntimes[name][plug.Version].State,
			})
		}
	}
	return pluginRuntimes, nil
}

func (p *Plugin) UpdatePlugin(name plugin.PluginName, version string, enabled bool, cfg any) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	pluginsCfgs, err := p.getPlugins(pluginsKey)
	if err != nil {
		logrus.Errorf("failed to get plugins: %+v", err)
		return err
	}
	for i, plug := range pluginsCfgs[name] {
		if plug.Version == version {
			if cfg == nil {
				cfg = plug.Config
			}
			pluginsCfgs[name][i] = plugin.PluginConfig{
				Path:        plug.Path,
				Version:     plug.Version,
				Enabled:     enabled,
				CreateAt:    plug.CreateAt,
				Symbol:      plug.Symbol,
				Description: plug.Description,
				Config:      cfg,
			}
			break
		}
	}
	data, err := json.Marshal(pluginsCfgs)
	if err != nil {
		logrus.Errorf("failed to marshal plugins: %+v", err)
		return err
	}
	storage.RedisClient().Set(fmt.Sprintf("%s:%s", p.redisBaseKey, pluginsKey), string(data), 0)
	return nil
}

func (p *Plugin) Stop() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	p.pm.Stop()
}

func (p *Plugin) getPlugins(key string) (map[plugin.PluginName][]plugin.PluginConfig, error) {
	rds := storage.RedisClient()
	plugins, err := rds.Get(fmt.Sprintf("%s:%s", p.redisBaseKey, key))
	if err != nil {
		logrus.Errorf("failed to get plugins: %+v", err)
		return nil, err
	}
	var cfg map[plugin.PluginName][]plugin.PluginConfig
	err = json.Unmarshal([]byte(plugins), &cfg)
	if err != nil {
		logrus.Errorf("failed to unmarshal plugins: %+v", err)
		return nil, err
	}
	return cfg, nil
}
