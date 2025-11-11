package cmd

import (
	"danmaku-tool/cmd/flags"
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/service"
	"danmaku-tool/internal/utils"
)

func Init() {
	// init config
	config.Init(flags.ConfigPath, flags.Debug)
	// init logger
	utils.InitLogger(flags.Debug, false)
	// initializers
	for _, init := range danmaku.GetInitializers() {
		if i, ok := init.(danmaku.Initializer); ok {
			if err := i.Init(); err != nil {
				utils.InfoLog("init", err.Error())
			}
		}
	}
}

func InitServer() {
	utils.InitLogger(flags.Debug, true)
	// server初始化必要资源
	for _, init := range danmaku.GetInitializers() {
		if i, ok := init.(danmaku.ServerInitializer); ok {
			if err := i.ServerInit(); err != nil {
				utils.ErrorLog("init_server", err.Error())
			}
		}
	}
}

func Release() {
	mode := service.GetDandanSourceMode()
	if re, ok := mode.(danmaku.Finalizer); ok {
		err := re.Finalize()
		if err != nil {
			utils.ErrorLog("release", err.Error())
		}
	}
}
