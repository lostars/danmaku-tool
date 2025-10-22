package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var (
	Debug     = false
	Port      = 8089
	RunServer = false
)

var danmakuConfig *DanmakuConfig

func GetConfig() *DanmakuConfig {
	if danmakuConfig != nil {
		return danmakuConfig
	}
	var file = loadDefaultConfig()
	if file == nil {
		panic("danmaku config file load failed")
	}
	if err := yaml.Unmarshal(file, &danmakuConfig); err != nil {
		panic(err.Error())
	}
	return danmakuConfig
}

func loadDefaultConfig() []byte {
	home, _ := os.UserHomeDir()
	if home != "" {
		// load from user home .config/danmaku-tool/config.yaml
		CfgPath := filepath.Join(home, ".config", "danmaku-tool", "config.yaml")
		file, _ := os.ReadFile(CfgPath)
		if file != nil {
			return file
		}
	}
	execPath, _ := os.Executable()
	if execPath != "" {
		CfgPath := filepath.Join(filepath.Dir(execPath), "config.yaml")
		file, _ := os.ReadFile(CfgPath)
		if file != nil {
			return file
		}
	}
	return nil
}

type DanmakuConfig struct {
	SavePath            string `yaml:"save-path"`
	MergeDanmakuInMills int64  `yaml:"merge-danmaku-in-mills"`
	Bilibili            struct {
		Cookie    string `yaml:"cookie"`
		MaxWorker int    `yaml:"max-worker"`
		Timeout   int64  `yaml:"timeout"` // in seconds
	} `yaml:"bilibili"`
}
