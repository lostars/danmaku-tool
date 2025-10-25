package cmd

import (
	"danmu-tool/cmd/flags"
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
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
	for _, init := range danmaku.ManagerOfDanmaku.Initializers {
		if err := init.Init(config.GetConfig()); err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "initialize error: %v", err)
		}
	}
}
