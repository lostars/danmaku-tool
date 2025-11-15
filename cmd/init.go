package cmd

import (
	"danmaku-tool/cmd/flags"
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/service"
	"danmaku-tool/internal/utils"

	"github.com/go-co-op/gocron/v2"
)

func Init() {
	// init config
	config.Init(flags.ConfigPath, flags.Debug)
	// init logger
	utils.InitLogger(flags.Debug, flags.JsonLogger)
	// initializers
	for _, init := range danmaku.Initializers() {
		if err := init.Init(); err != nil {
			utils.InfoLog(initC, err.Error())
		}
	}
}

func InitServer() {
	flags.JsonLogger = true
	Init()
	// server初始化必要资源
	for _, init := range danmaku.ServerInitializers() {
		if err := init.ServerInit(); err != nil {
			utils.ErrorLog(initServerC, err.Error())
		}
	}
	// 初始化任务
	go func() {
		s, err := gocron.NewScheduler()
		if err != nil {
			utils.ErrorLog(initServerC, err.Error())
			return
		}
		for _, j := range danmaku.Jobs() {
			if e := j.CreateJob(s); e != nil {
				utils.ErrorLog(initServerC, e.Error())
			}
		}
		scheduler = s
		scheduler.Start()
	}()
}

func Release() {
	mode := service.GetDandanSourceMode()
	if re, ok := mode.(danmaku.Finalizer); ok {
		err := re.Finalize()
		if err != nil {
			utils.ErrorLog(releaseC, err.Error())
		}
	}
	if scheduler != nil {
		if err := scheduler.Shutdown(); err != nil {
			utils.ErrorLog(releaseC, err.Error())
		}
	}
}

const (
	releaseC    = "release"
	initServerC = "init_server"
	initC       = "init"
)
