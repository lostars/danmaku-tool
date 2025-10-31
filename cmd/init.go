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
		if err := init.Init(); err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "initialize info: %v\n", err)
		}
	}
}

func Release() {
	mode := service.GetDandanSourceMode()
	if re, ok := mode.(service.SourceRelease); ok {
		err := re.ReleaseSource()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "release source error: %v\n", err)
		}
	}
}
