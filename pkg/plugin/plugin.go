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
	"sync"
)

type PluginName string

type PluginState string

const (
	PluginStateCreate  PluginState = "create"
	PluginStateRunning PluginState = "running"
	PluginStateStopped PluginState = "stopped"
)

type Plugin interface {
	Run() // run the plugin
	GetName() PluginName
	Stop()                 // stop the plugin
	Health() PluginState   // get the health of the plugin
	RefreshConfig(cfg any) // refresh the config of the plugin
}

type BasePlugin struct {
	Plugin
	Context context.Context
	Cancel  context.CancelFunc
	Name    PluginName
	State   PluginState
	Config  any
	Mu      *sync.RWMutex
}

func (p *BasePlugin) ParseConfig(cfg any) error {
	p.Mu.RLock()
	defer p.Mu.RUnlock()
	str, err := json.Marshal(p.Config)
	if err != nil {
		return err
	}
	err = json.Unmarshal(str, cfg)
	if err != nil {
		return err
	}
	return nil
}

func (p *BasePlugin) SetConfig(cfg any) {
	p.Mu.Lock()
	defer p.Mu.Unlock()
	p.Config = cfg
}
