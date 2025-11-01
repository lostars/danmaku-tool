package cmd

import (
	"danmu-tool/cmd/flags"
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"danmu-tool/internal/service"
	"danmu-tool/internal/utils"
	"fmt"
	"os"
)

func Init() {
	// init config
	config.Init(flags.ConfigPath, flags.Debug)
	// init logger
	utils.LoggerConf.InitLogger(flags.Debug)
	// initializers
	for _, init := range danmaku.GetInitializers() {
		if i, ok := init.(danmaku.Initializer); ok {
			if err := i.Init(); err != nil {
				_, _ = fmt.Fprintf(os.Stdout, "initialize info: %v\n", err)
			}
		}
	}
}

func InitServer() {
	for _, init := range danmaku.GetInitializers() {
		if i, ok := init.(danmaku.ServerInitializer); ok {
			if err := i.ServerInit(); err != nil {
				_, _ = fmt.Fprintf(os.Stdout, "server initialize info: %v\n", err)
			}
		}
	}
}

func Release() {
	mode := service.GetDandanSourceMode()
	if re, ok := mode.(danmaku.Finalizer); ok {
		err := re.Finalize()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "release source error: %v\n", err)
		}
	}
}
