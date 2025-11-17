package cmd

import (
	"danmaku-tool/cmd/flags"
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
)

func Init() {
	// init config
	config.Init(flags.ConfigPath, flags.Debug)
	// init logger
	utils.InitLogger(flags.Debug, flags.JsonLogger)
	// initializers
	for _, init := range danmaku.Initializers() {
		if err := init.Init(); err != nil {
			utils.ErrorLog(initC, err.Error())
		}
	}
}

func InitServer() {
	flags.JsonLogger = true
	Init()
	// server初始化必要资源
	for _, init := range danmaku.ServerInitializers() {
		if async, ok := init.(danmaku.AsyncServerInitializer); ok {
			async := async
			go func() {
				if err := async.AsyncInit(); err != nil {
					utils.ErrorLog(initServerC, err.Error())
				}
			}()
		} else {
			if err := init.ServerInit(); err != nil {
				utils.ErrorLog(initServerC, err.Error())
			}
		}
	}
}

func Release() {
	for _, f := range danmaku.Finalizers() {
		if err := f.Finalize(); err != nil {
			utils.ErrorLog(releaseC, err.Error())
		}
	}
}

const (
	releaseC    = "release"
	initServerC = "init_server"
	initC       = "init"
)
