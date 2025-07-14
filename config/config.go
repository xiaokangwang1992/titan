/*
 @Version : 1.0
 @Author  : steven.wang
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2022/2022/20 20/19/22
 @Desc    :
*/

package config

import (
	"os"

	"github.com/piaobeizu/titan/pkg/utils"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

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
	Sses        []string `yaml:"sses,omitempty"`
	WebSockets  []string `yaml:"websockets,omitempty"`
}

type WebGroup struct {
	Uri  string `yaml:"uri,omitempty"`
	Page string `yaml:"page,omitempty"`
}

type Http struct {
	ApiAddr     string                    `yaml:"api-addr,omitempty"`
	Version     string                    `yaml:"version,omitempty"`
	Route       string                    `yaml:"route,omitempty"`
	Routes      map[string]ApiGroup       `yaml:"routes,omitempty"`
	Swagger     bool                      `yaml:"swagger,omitempty"`
	Middlewares map[string]map[string]any `yaml:"middlewares,omitempty"`
	WebAddr     string                    `yaml:"web-addr,omitempty"`
}

type Grpc struct {
	ListenAddr string `yaml:"listen-addr,omitempty"`
}

type FileUploader struct {
	FileMaxSize int64       `yaml:"file-max-size,omitempty"`
	ChunkSize   int64       `yaml:"chunk-size,omitempty"`
	BufferSize  int64       `yaml:"buffer-size,omitempty"`
	FileTypes   []string    `yaml:"file-types,omitempty"`
	FormName    string      `yaml:"form-name,omitempty"`
	ExpireTime  int64       `yaml:"expire-time,omitempty"`
	PathMode    os.FileMode `yaml:"path-mode,omitempty"`
}

type FileSystem struct {
	FileUploader *FileUploader `yaml:"file-uploader,omitempty"`
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

type Event struct {
	MsgSize int `yaml:"msg_size" default:"1000"`
}

type Plugin struct {
	Refresh          int    `yaml:"refresh,omitempty"`
	Config           string `yaml:"config,omitempty"`
	GracefulShutdown int    `yaml:"graceful-shutdown,omitempty"`
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
	FileSystem *FileSystem `yaml:"filesystem,omitempty"`
	Schedulers []Scheduler `yaml:"schedulers,omitempty"`
	Business   any         `yaml:"business,omitempty"`
	Ants       *Ants       `yaml:"ants,omitempty"`
	Event      *Event      `yaml:"event,omitempty"`
	Plugin     *Plugin     `yaml:"plugin,omitempty"`
}

var (
	cfg    Config
	routes map[string]ApiGroup
)

func getConfig(f string) (Config, error) {
	var ret Config
	if f == "" {
		f = os.Getenv("APP_CONFIG_PATH")
	}
	err := utils.ReadFileToStruct(f, &ret, "yaml", false)
	return ret, err
}

func getRoutes() (map[string]ApiGroup, error) {
	var (
		err error
		ret = cfg.Http.Routes
	)
	if ret == nil {
		err = utils.ReadFileToStruct(cfg.Http.Route, &ret, "yaml", false)
	}
	return ret, err
}

func InitCfg(f string) (err error) {
	cfg, err = getConfig(f)
	if err != nil {
		return
	}
	logrus.Infof("get config: %+v", cfg)
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

func GetBusinessConfig(business any) error {
	content, err := yaml.Marshal(cfg.Business)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(content, business)
}
