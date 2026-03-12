package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	Version  string
	ConfPath string
)

var danmakuConfig *DanmakuConfig

func Init(path string) {
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
	// init tz
	if danmakuConfig.Timezone == "" {
		if p := os.Getenv(timeZoneEnv); p != "" {
			danmakuConfig.Timezone = p
		} else {
			danmakuConfig.Timezone = "Asia/Shanghai"
		}
	}
	if danmakuConfig.DanDan.DisableAuth {
		fmt.Println("dandan API authentication is DISABLED!!!")
	}
	// init replace rules
	titleMatchRules = make(map[string]*TitleMatchRule, len(danmakuConfig.Tokenizer.TitleMatchList))
	for _, r := range danmakuConfig.Tokenizer.TitleMatchList {
		titleMatchRules[r.Title+"\x00"+r.Platform] = &r
	}
	// init year match rules
	yearMatchRules = make(map[string]*YearMatchRule, len(danmakuConfig.Tokenizer.YearMatchList))
	for _, y := range danmakuConfig.Tokenizer.YearMatchList {
		yearMatchRules[y.Title] = &y
	}
}

func MatchTitleRule(title string, platform string) *TitleMatchRule {
	return titleMatchRules[title+"\x00"+platform]
}

func MatchYearRule(title string) *YearMatchRule {
	return yearMatchRules[title]
}

var titleMatchRules map[string]*TitleMatchRule
var yearMatchRules map[string]*YearMatchRule

func GetConfig() *DanmakuConfig {
	return danmakuConfig
}

const configPathEnv = "DANMAKU_TOOL_CONFIG"
const timeZoneEnv = "TZ"

func loadFromPath(path string) []byte {
	file, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	ConfPath = path
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

func GetPlatformConfig(platform string) *PlatformConfig {
	for _, v := range GetConfig().Platforms {
		if v.Name == platform {
			return &v
		}
	}
	return nil
}

type DanmakuConfig struct {
	Timezone  string           `yaml:"timezone"`
	Debug     bool             `yaml:"debug"`
	SavePath  string           `yaml:"save-path"`
	DanDan    DandanConfig     `yaml:"dandan"`
	UA        string           `yaml:"ua"`
	Platforms []PlatformConfig `yaml:"platforms"`
	Emby      EmbyConfig       `yaml:"emby"`
	Server    ServerConfig     `yaml:"server"`
	Tokenizer TokenizerConfig  `yaml:"tokenizer"`
}

type DandanConfig struct {
	CacheTimeout int    `yaml:"cache-timeout"`
	Mode         string `yaml:"mode"`
	Timeout      int    `yaml:"timeout"`
	DisableAuth  bool   `yaml:"disable-auth"` // 是否禁用API验证
}

func GetDandan() *DandanConfig {
	return &GetConfig().DanDan
}

type TokenizerConfig struct {
	TitleMatchList []TitleMatchRule `yaml:"title-match-list"`
	YearMatchList  []YearMatchRule  `yaml:"year-match-list"`
	Rematch        []Rematch        `yaml:"rematch"`
}

type YearMatchRule struct {
	Title  string `yaml:"title"`
	Season *int   `yaml:"season"`
	Year   int    `yaml:"year"`
}

type TitleMatchRule struct {
	Title       string `yaml:"title"`
	Replacement string `yaml:"replacement"`
	Platform    string `yaml:"platform"`
	Mode        string `yaml:"mode"`
}

type Rematch struct {
	Platform string `yaml:"platform"`
	MediaId  string `yaml:"media-id"`
	Targets  []struct {
		Item   string `yaml:"item"`
		Union  bool   `yaml:"union"`
		Source string `yaml:"source"`
	} `yaml:"targets"`
	Episodes []struct {
		Season  int    `yaml:"season"`
		Episode string `yaml:"episode"`
	} `yaml:"episodes"`
}

func (t TokenizerConfig) MediaRematch(platform string, mediaId string, seasonId int) bool {
	if platform == "" || mediaId == "" {
		return false
	}
	for _, re := range t.Rematch {
		if len(re.Targets) > 0 && mediaId == re.MediaId {
			return true
		}
		if re.Platform == platform && mediaId == re.MediaId {
			for _, ep := range re.Episodes {
				if ep.Season == seasonId {
					return true
				}
			}
		}
	}
	return false
}

type EmbyConfig struct {
	Url   string `yaml:"url"`
	User  string `yaml:"user"`
	Token string `yaml:"token"`
}

type ServerConfig struct {
	Port    int      `yaml:"port"`    // can be overwritten by cli parameter
	Timeout int      `yaml:"timeout"` // 全局api超时时间
	Tokens  []string `yaml:"tokens"`  // token配置
}

type PlatformConfig struct {
	Name                string   `yaml:"name"`
	Priority            int      `yaml:"priority"`
	Cookie              string   `yaml:"cookie"`
	MaxWorker           int      `yaml:"max-worker"`
	Timeout             int64    `yaml:"timeout"` // in seconds
	MergeDanmakuInMills int64    `yaml:"merge-danmaku-in-mills"`
	Persists            []string `yaml:"persists"`
	PersistExpire       int      `yaml:"persist-expire"`
}

func (p PlatformConfig) Disabled() bool {
	return p.Priority < 0
}

func (p PlatformConfig) FileExpire(end time.Time) bool {
	if p.PersistExpire <= 0 {
		return false
	}
	return int(time.Since(end).Seconds()) > p.PersistExpire
}
