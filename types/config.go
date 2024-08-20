/*
 @Version : 1.0
 @Author  : steven.wang
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2022/2022/20 20/19/22
 @Desc    :
*/

package types

import (
	"os"

	"github.com/piaobeizu/titan/utils"
)

type Mysql struct {
	MysqlHost string `yaml:"host" default:""`
	MysqlPort uint32 `yaml:"port" default:""`
	MysqlUser string `yaml:"user" default:""`
	MysqlPass string `yaml:"password" default:""`
	MysqlDB   string `yaml:"db" default:""`
}

type Redis struct {
	RedisHost     string `yaml:"host"`
	RedisPort     uint32 `yaml:"port" default:"6379"`
	RedisUser     string `yaml:"user"`
	RedisPassword string `yaml:"password"`
	RedisDB       int    `yaml:"db" default:"0"`
}

type Route struct {
	Uri     string `yaml:"uri,omitempty"`
	Method  string `yaml:"method,omitempty"`
	Handler string `yaml:"handler,omitempty"`
}
type Middleware struct {
	Name string         `yaml:"name,omitempty"`
	Args map[string]any `yaml:"args,omitempty"`
}

type ApiGroup struct {
	Middlewares []string `yaml:"middlewares" default:"DefaultMiddleware"`
	Routers     []string `yaml:"routers,omitempty"`
}

type WebGroup struct {
	Uri  string `yaml:"uri,omitempty"`
	Page string `yaml:"page,omitempty"`
}

type Http struct {
	ApiAddr     string                    `yaml:"api-addr,omitempty"`
	Route       string                    `yaml:"route,omitempty"`
	Routes      map[string]ApiGroup       `yaml:"routes,omitempty"`
	Swagger     bool                      `yaml:"swagger,omitempty"`
	Middlewares map[string]map[string]any `yaml:"middlewares,omitempty"`
	WebAddr     string                    `yaml:"web-addr,omitempty"`
}

type Grpc struct {
	ListenAddr string `yaml:"listen-addr,omitempty"`
}

type Scheduler struct {
	Enabled bool                   `yaml:"enabled,omitempty"`
	Name    string                 `yaml:"name,omitempty"`
	Cron    string                 `yaml:"cron,omitempty"`
	Detail  string                 `yaml:"detail,omitempty"`
	Method  string                 `yaml:"method,omitempty"`
	Args    map[string]interface{} `yaml:"args,omitempty"`
}

type Ants struct {
	Size int `yaml:"size" default:"100"`
}

type Config struct {
	Mode       string      `yaml:"mode" default:"debug"`
	Name       string      `yaml:"name" default:""`
	LogMode    string      `yaml:"log-mode" default:"debug"`
	Http       *Http       `yaml:"http,omitempty"`
	Grpc       *Grpc       `yaml:"grpc,omitempty"`
	Mysql      string      `yaml:"mysql,omitempty"`
	Redis      string      `yaml:"redis,omitempty"`
	Oss        string      `yaml:"oss,omitempty"`
	Schedulers []Scheduler `yaml:"schedulers,omitempty"`
	Business   any         `yaml:"business,omitempty"`
	Ants       *Ants       `yaml:"ants,omitempty"`
}

var (
	cfg    Config
	routes map[string]ApiGroup
)

func getConfig() (Config, error) {
	var ret Config
	f := os.Getenv("APP_CONFIG_PATH")
	err := utils.ReadFileToStruct(f, &ret, "yaml")
	return ret, err
}

func getRoutes() (map[string]ApiGroup, error) {
	var (
		err error
		ret = cfg.Http.Routes
	)
	if ret == nil {
		err = utils.ReadFileToStruct(cfg.Http.Route, &ret, "yaml")
	}
	return ret, err
}

func InitCfg() (err error) {
	cfg, err = getConfig()
	if err != nil {
		return
	}
	return initRoutes()
}
func initRoutes() (err error) {
	routes, err = getRoutes()
	return
}

func GetConfig() *Config {
	return &cfg
}

func GetRoutes() map[string]ApiGroup {
	return routes
}
