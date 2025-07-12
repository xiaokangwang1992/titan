/*
 * @Version : 1.0
 * @Author  : xiaokang.w
 * @Email   : xiaokang.w@gmicloud.ai
 * @Date    : 2025/07/12
 * @Desc    : 描述信息
 */

package plugin

import "context"

type PluginName string

type PluginState string

const (
	PluginStateCreate  PluginState = "create"
	PluginStateInit    PluginState = "init"
	PluginStateRunning PluginState = "running"
	PluginStateStopped PluginState = "stopped"
	PluginStatePaused  PluginState = "paused"
)

type PluginInterface interface {
	Init() error // init the plugin
	Run()        // run the plugin
	GetName() PluginName
	Stop()                 // stop the plugin
	Health() PluginState   // get the health of the plugin
	RefreshConfig(cfg any) // refresh the config of the plugin
}

type BasePlugin struct {
	Context context.Context
	Cancel  context.CancelFunc
	Name    PluginName
	Version string
	Config  any
}

func (b *BasePlugin) Init() error {
	return nil
}
