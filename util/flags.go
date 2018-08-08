package util

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"gopkg.in/urfave/cli.v1"
)

var (
	ConfigFileFlag = cli.StringFlag{
		Name:  "config",
		Usage: "config file",
		Value: "../config.toml",
	}
	CPUProfileFlag = cli.StringFlag{
		Name:  "cpu-profile",
		Usage: "write cpu profile to file",
	}
	MemProfileFlag = cli.StringFlag{
		Name:  "mem-profile",
		Usage: "write memory profile to file",
	}
)

var Flags = map[string]cli.Flag{
	"p2p_port": cli.IntFlag{
		Name:  "p2p_port",
		Usage: "p2p network port",
	},
	"http_port": cli.IntFlag{
		Name:  "http_port",
		Usage: "http rpc network port",
	},
	"dbpath": cli.StringFlag{
		Name:  "dbpath",
		Usage: "braft leveldb path",
	},
	"bch_host": cli.StringFlag{
		Name:  "bch-host",
		Usage: "the bch host that the node watch",
	},
	"bch_height": cli.Int64Flag{
		Name:  "bch-height",
		Usage: "start watching height",
	},
	"loglevel": cli.StringFlag{
		Name:  "loglevel",
		Usage: "log level, debug, info, warn, error or crti",
	},
}

// NewApp creates an APP
func NewApp() *cli.App {
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Author = ""
	app.Email = ""
	app.Version = "0.0.1"
	app.Usage = "braft test"
	return app
}

// GetConfigFile 获取config文件
func GetConfigFile(ctx *cli.Context) string {
	return ctx.GlobalString(ConfigFileFlag.Name)
}

// GetCPUProfile 获取CPU profile的保存文件路径
func GetCPUProfile(ctx *cli.Context) string {
	return ctx.GlobalString(CPUProfileFlag.Name)
}

// GetMemProfile 获取内存分析的保存文件路径
func GetMemProfile(ctx *cli.Context) string {
	return ctx.GlobalString(MemProfileFlag.Name)
}

// ReadConfigToViper 把命令行传入的参数保存到viper配置里面
func ReadConfigToViper(ctx *cli.Context) {
	for _, flag := range Flags {
		switch flag := flag.(type) {
		case cli.StringFlag:
			value := ctx.GlobalString(flag.Name)
			if len(value) > 0 {
				viper.Set("DGW."+flag.Name, value)
			}
		case cli.IntFlag:
			value := ctx.GlobalInt(flag.Name)
			if value != 0 {
				viper.Set("DGW."+flag.Name, value)
			}
		case cli.Int64Flag:
			value := ctx.GlobalInt64(flag.Name)
			if value != 0 {
				viper.Set("DGW."+flag.Name, value)
			}
		}
	}
}
