package danmaku

import (
	"danmu-tool/internal/config"
	"fmt"
)

func PlatformError(p Platform, text string) error {
	return fmt.Errorf("[%s] %s", p, text)
}

func DataPersistError(d DataPersistType, text string) error {
	return fmt.Errorf("[%s] %s", d, text)
}

type MediaType string

const (
	Series = "series" // 季
	Movie  = "movie"  // 单集
)

type Media struct {
	Type     MediaType
	TypeDesc string // 类型描述 TV动画 / 综艺
	Id       string // 存储平台实际id
	Title    string
	Desc     string
	Episodes []*MediaEpisode
}

type MediaEpisode struct {
	Id        string // 存储平台实际的id
	EpisodeId string // 第几话
	Title     string

	Danmaku []*StandardDanmaku // 弹幕信息
}

type Scraper interface {
	Platform() Platform
	// Scrape 抓取并保存弹幕
	Scrape(id interface{}) error
}

type Initializer interface {
	Init(conf *config.DanmakuConfig) error
}

type MediaSearcher interface {
	// Search 搜索剧集信息，不会获取ep
	Search(keyword string) ([]*Media, error)
	// GetDanmaku 实时获取平台弹幕 id: [platform]_[id]_[id]
	GetDanmaku(id string) ([]*StandardDanmaku, error)
	SearcherType() Platform
}

type DataPersist interface {
	WriteToFile(fullPath, filename string) error
	Type() DataPersistType
}

// https://api.dandanplay.net/swagger/index.html#/%E5%BC%B9%E5%B9%95/Comment_GetComment
// p 出现时间,模式,颜色,用户ID

type StandardDanmaku struct {
	Offset int64 // 偏移量 ms 注意dandan中保存的是秒，保留2位小数，这里为了精度使用ms，在API返回或者写入时才进行转换
	Mode   int   // 1滚动 4底部 5顶部
	Color  int   // 颜色 数字格式 16777215
	// 以上三个字段按照顺序兼容dandan API p字段

	Content string // dandan API m字段

	// 以下字段用于其他记录
	FontSize int32 // 字体大小
	Platform Platform
}

const RollMode = 1
const BottomMode = 4
const TopMode = 5

type Manager struct {
	Scrapers     map[string]Scraper
	Searchers    map[string]MediaSearcher
	Initializers []Initializer
}

var ManagerOfDanmaku = &Manager{
	Scrapers:     map[string]Scraper{},
	Searchers:    map[string]MediaSearcher{},
	Initializers: []Initializer{},
}

func (m *Manager) GetPlatforms() []string {
	var result []string
	for _, v := range m.Scrapers {
		result = append(result, string(v.Platform()))
	}
	return result
}

func Register(i interface{}) {
	// TODO map相同名称多次注入会被覆盖
	if v, ok := i.(MediaSearcher); ok {
		ManagerOfDanmaku.Searchers[string(v.SearcherType())] = v
	}
	if v, ok := i.(Scraper); ok {
		ManagerOfDanmaku.Scrapers[string(v.Platform())] = v
	}
	if v, ok := i.(Initializer); ok {
		ManagerOfDanmaku.Initializers = append(ManagerOfDanmaku.Initializers, v)
	}
}

type Platform string

const (
	Bilibili = "bilibili"
	Tencent  = "tencent"
)

type DataPersistType string

const (
	XMLPersistType = "xml"
)
