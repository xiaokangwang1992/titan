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
	Config  any
}
