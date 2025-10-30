package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var (
	Version string
)

var danmakuConfig *DanmakuConfig

func Init(path string, debug bool) {
	if danmakuConfig != nil {
		return
	}
	var file = loadDefaultConfig(path)
	if file == nil {
		panic("danmaku config file load failed")
	}
	if err := yaml.Unmarshal(file, &danmakuConfig); err != nil {
		panic(err.Error())
	}
	danmakuConfig.Debug = debug
}

func GetConfig() *DanmakuConfig {
	return danmakuConfig
}

const configPathEnv = "DANMAKU_TOOL_CONFIG"

func loadFromPath(path string) []byte {
	file, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return file
}

func loadDefaultConfig(path string) []byte {
	// load from cmd parameter
	if path != "" {
		return loadFromPath(path)
	}
	// load from env
	if p := os.Getenv(configPathEnv); p != "" {
		return loadFromPath(p)
	}
	if home, _ := os.UserHomeDir(); home != "" {
		// load from user home .config/danmaku-tool/config.yaml
		CfgPath := filepath.Join(home, ".config", "danmaku-tool", "config.yaml")
		return loadFromPath(CfgPath)
	}
	if execPath, _ := os.Executable(); execPath != "" {
		CfgPath := filepath.Join(filepath.Dir(execPath), "config.yaml")
		return loadFromPath(CfgPath)
	}
	return nil
}

func (c *DanmakuConfig) GetPlatformConfig(platform string) *PlatformConfig {
	for _, v := range c.Platforms {
		if v.Name == platform {
			return &v
		}
	}
	return nil
}

type DanmakuConfig struct {
	Debug         bool             `yaml:"debug"`
	SavePath      string           `yaml:"save-path"`
	DandanMode    string           `yaml:"dandan-mode"`
	DandanTimeout int              `yaml:"dandan-timeout"`
	UA            string           `yaml:"ua"`
	Platforms     []PlatformConfig `yaml:"platforms"`
	Emby          EmbyConfig       `yaml:"emby"`
	Server        ServerConfig     `yaml:"server"`
}

type EmbyConfig struct {
	Url   string `yaml:"url"`
	User  string `yaml:"user"`
	Token string `yaml:"token"`
}

func (c *DanmakuConfig) EmbyEnabled() bool {
	return c.Emby.User != "" && c.Emby.Url != "" && c.Emby.Token != ""
}

type ServerConfig struct {
	Port    int      `yaml:"port"`    // can be overwritten by cli parameter
	Timeout int      `yaml:"timeout"` // 全局api超时时间
	Tokens  []string `yaml:"tokens"`  // token配置
}

type DanmakuPersistConfig struct {
	Indent   bool   `yaml:"indent"`
	Compress bool   `yaml:"compress"`
	Type     string `yaml:"type"`
}

type PlatformConfig struct {
	Name                string                 `yaml:"name"`
	Priority            int                    `yaml:"priority"`
	Cookie              string                 `yaml:"cookie"`
	MaxWorker           int                    `yaml:"max-worker"`
	Timeout             int64                  `yaml:"timeout"` // in seconds
	MergeDanmakuInMills int64                  `yaml:"merge-danmaku-in-mills"`
	Persists            []DanmakuPersistConfig `yaml:"persists"`
}
