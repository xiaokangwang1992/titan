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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/GMISWE/ieops-plugins/event"
	"github.com/GMISWE/ieops-plugins/plugin"
	"github.com/panjf2000/ants/v2"
	"github.com/piaobeizu/titan/pkg/storage"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
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
	p          *Plugin
)

func NewPlugin(ctx context.Context, opts ...PluginOption) *Plugin {
	if p != nil {
		return p
	}
	opt := loadOptions(opts...)
	if opt.refresh == 0 {
		opt.refresh = 10
	}
	if opt.redisBaseKey == "" {
		opt.redisBaseKey = "plugins"
	}

	p = &Plugin{
		ctx:          ctx,
		mu:           sync.RWMutex{},
		refresh:      opt.refresh,
		redisBaseKey: opt.redisBaseKey,
		pm: plugin.NewPluginManager(
			ctx, plugin.WithPool(opt.pool),
			plugin.WithEvent(opt.event),
			plugin.WithConfig(opt.conf),
		),
	}
	return p
}

func (p *Plugin) Start() {
	pm := plugin.NewPluginManager(p.ctx)
	ticker := time.NewTicker(time.Second * time.Duration(p.refresh))
	ticker1 := time.NewTicker(time.Second * 5)
	defer ticker.Stop()
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker1.C:
			rts := p.pm.GetAllRuntimes()
			runnings, stoppings, stopped, errs := 0, 0, 0, 0
			for _, runtime := range rts {
				if runtime.GetState() == "running" {
					runnings++
				}
				if runtime.GetState() == "stopping" {
					stoppings++
				}
				if runtime.GetState() == "stopped" {
					stopped++
				}
				if runtime.GetState() == "error" {
					errs++
				}
			}
			logrus.Infof("plugin statistics - running: %d, stopping: %d, stopped: %d, error: %d", runnings, stoppings, stopped, errs)
			p.pm.Table()
		case <-ticker.C:
			pluginsCfgs, err := p.GetPluginsFromRedis(pluginsKey)
			if err != nil {
				logrus.Warnf("failed to get plugins: %+v", err)
				continue
			}
			for name, plugins := range pluginsCfgs {
				for _, plug := range plugins {
					if plug.Enabled {
						if err := pm.StartPlugin(&plugin.PluginDefinition{
							PluginName:  name,
							PluginPath:  plug.Path,
							Version:     plug.Version,
							Graceful:    plug.Graceful,
							ModifyAt:    plug.CreateAt,
							SymbolName:  plug.Symbol,
							Description: plug.Description,
							Config:      plug.Config,
						}); err != nil {
							logrus.Errorf("failed to start plugin: %+v", err)
							continue
						}
					} else {
						if err := pm.StopPlugin(name, plug.Version); err != nil && err != plugin.ErrPluginNotFound {
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

	pluginsCfgs, err := p.GetPluginsFromRedis(pluginsKey)
	if err != nil {
		logrus.Errorf("failed to get plugins: %+v", err)
		return err
	}
	// Defensive check to ensure pluginsCfgs is not nil
	if pluginsCfgs == nil {
		pluginsCfgs = make(map[plugin.PluginName][]plugin.PluginConfig)
	}
	for _, plug := range pluginsCfgs[define.PluginName] {
		if plug.Version == define.Version {
			return fmt.Errorf("plugin: %s already exists", define.PluginName)
		}
	}
	if _, ok := pluginsCfgs[define.PluginName]; !ok {
		pluginsCfgs[define.PluginName] = make([]plugin.PluginConfig, 0)
	}
	pluginsCfgs[define.PluginName] = append(pluginsCfgs[define.PluginName], plugin.PluginConfig{
		Path:        define.PluginPath,
		Version:     define.Version,
		Enabled:     true,
		CreateAt:    define.ModifyAt,
		Symbol:      define.SymbolName,
		Description: define.Description,
		Md5:         define.Md5,
		Config:      define.Config,
	})
	data, err := yaml.Marshal(pluginsCfgs)
	if err != nil {
		logrus.Errorf("failed to marshal plugins: %+v", err)
		return err
	}
	storage.RedisClient().Set(fmt.Sprintf("%s%s", p.redisBaseKey, pluginsKey), string(data), 0)
	return nil
}

// func (p *Plugin) StopPlugin(name plugin.PluginName, version string) error {
// 	p.mu.RLock()
// 	defer p.mu.RUnlock()
// 	pluginsCfgs, err := p.getPlugins(pluginsKey)
// 	if err != nil {
// 		logrus.Errorf("failed to get plugins: %+v", err)
// 		return err
// 	}
// 	for i, plug := range pluginsCfgs[name] {
// 		if plug.Version == version {
// 			pluginsCfgs[name][i].Enabled = false
// 			break
// 		}
// 	}
// 	data, err := json.Marshal(pluginsCfgs)
// 	if err != nil {
// 		logrus.Errorf("failed to marshal plugins: %+v", err)
// 		return err
// 	}
// 	storage.RedisClient().Set(fmt.Sprintf("%s:%s", p.redisBaseKey, pluginsKey), string(data), 0)
// 	return nil
// }

func (p *Plugin) DeletePlugin(name plugin.PluginName, version string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	pluginsCfgs, err := p.GetPluginsFromRedis(pluginsKey)
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
	data, err := yaml.Marshal(pluginsCfgs)
	if err != nil {
		logrus.Errorf("failed to marshal plugins: %+v", err)
		return err
	}
	if err = p.pm.DeletePlugin(name, version); err != nil {
		logrus.Errorf("failed to delete plugin: %+v", err)
		return err
	}
	storage.RedisClient().Set(fmt.Sprintf("%s%s", p.redisBaseKey, pluginsKey), string(data), 0)
	return nil
}

func (p *Plugin) ListPlugins() (map[plugin.PluginName][]*PluginRuntime, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	pluginCfgs, err := p.GetPluginsFromRedis(pluginsKey)
	if err != nil {
		logrus.Errorf("failed to get plugins: %+v", err)
		return nil, err
	}
	allRuntimes := p.pm.GetAllRuntimes()
	pluginRuntimes := make(map[plugin.PluginName][]*PluginRuntime)
	for name, plugins := range pluginCfgs {
		for _, plug := range plugins {
			// 安全地获取插件状态，避免空指针访问
			var state string
			if runtime, exists := allRuntimes[plugin.GetUniqueKey(name, plug.Version)]; exists {
				state = runtime.GetState()
			} else {
				state = "stopped" // 如果运行时不存在，默认为停止状态
			}

			pluginRuntimes[name] = append(pluginRuntimes[name], &PluginRuntime{
				Path:        plug.Path,
				Version:     plug.Version,
				Enabled:     plug.Enabled,
				CreateAt:    plug.CreateAt,
				Symbol:      plug.Symbol,
				Description: plug.Description,
				Config:      plug.Config,
				State:       state,
			})
		}
	}
	return pluginRuntimes, nil
}

func (p *Plugin) UpdatePlugin(name plugin.PluginName, version string, enabled bool, cfg any) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	pluginsCfgs, err := p.GetPluginsFromRedis(pluginsKey)
	if err != nil {
		logrus.Errorf("failed to get plugins: %+v", err)
		return err
	}
	exist := false
	for i, plug := range pluginsCfgs[name] {
		if plug.Version == version {
			if cfg == nil {
				cfg = plug.Config
			}
			pluginsCfgs[name][i].Enabled = enabled
			pluginsCfgs[name][i].Config = cfg
			exist = true
			break
		}
	}
	if !exist {
		return plugin.ErrPluginNotFound
	}
	data, err := yaml.Marshal(pluginsCfgs)
	if err != nil {
		logrus.Errorf("failed to marshal plugins: %+v", err)
		return err
	}
	storage.RedisClient().Set(fmt.Sprintf("%s%s", p.redisBaseKey, pluginsKey), string(data), 0)
	return nil
}

func (p *Plugin) AddEventFunc(name event.EventName, action event.EventAction, f func(param any) error) {
	p.pm.AddEventFunc(action, f)
}

func (p *Plugin) GetPluginMeta(name plugin.PluginName, version string) ([]plugin.PluginMeta, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	pluginCfgs, err := p.GetPluginsFromRedis(pluginsKey)
	if err != nil {
		logrus.Errorf("failed to get plugins: %+v", err)
		return nil, err
	}
	for _, plug := range pluginCfgs[name] {
		if plug.Version == version {
			metas, err := p.pm.GetPluginMeta(name, version)
			if err != nil {
				logrus.Errorf("failed to load plugin: %+v", err)
				return nil, err
			}
			return metas, nil
		}
	}
	return nil, plugin.ErrPluginNotFound
}

func (p *Plugin) GetPluginLogs(name plugin.PluginName, version string, taskID string, log chan string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pm.GetPluginLogs(name, version, taskID, log)
}

func (p *Plugin) GetPluginManager() *plugin.PluginManager {
	return p.pm
}

func (p *Plugin) Stop() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	p.pm.Stop()
}

func (p *Plugin) GetPluginsFromRedis(key string) (map[plugin.PluginName][]plugin.PluginConfig, error) {
	rds := storage.RedisClient()
	if !rds.Exists(fmt.Sprintf("%s%s", p.redisBaseKey, key)) {
		return make(map[plugin.PluginName][]plugin.PluginConfig), nil
	}
	plugins, err := rds.Get(fmt.Sprintf("%s%s", p.redisBaseKey, key))
	if err != nil {
		if !strings.Contains(err.Error(), "key not found") {
			ps := map[plugin.PluginName][]plugin.PluginConfig{}
			psStr, err := yaml.Marshal(ps)
			if err != nil {
				logrus.Errorf("failed to marshal plugins: %+v", err)
				return nil, err
			}
			rds.Set(fmt.Sprintf("%s%s", p.redisBaseKey, key), string(psStr), 0)
			plugins = string(psStr)
			// Continue to unmarshal the empty plugins map
		} else {
			logrus.Errorf("failed to get plugins: %+v", err)
			return nil, err
		}
	}
	var cfg map[plugin.PluginName][]plugin.PluginConfig
	err = yaml.Unmarshal([]byte(plugins), &cfg)
	return cfg, err
}
