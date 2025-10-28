package danmaku

import (
	"danmu-tool/internal/config"
	"fmt"
	"regexp"
	"strings"
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
	Platform Platform
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
	// Match 匹配剧集信息，如果是剧集，会获取ep信息同时返回 关键字格式是 'xxx S01E01'
	Match(keyword string) ([]*Media, error)
	// GetDanmaku 实时获取平台弹幕 id: [platform]_[id]_[id]
	GetDanmaku(id string) ([]*StandardDanmaku, error)
	SearcherType() Platform
}

var SeriesRegex = regexp.MustCompile("(.*)\\sS(\\d{1,3})E(\\d{1,3})$")
var ChineseNumber = "一|二|三|四|五|六|七|八|九|十|十一|十二|十三|十四|十五|十六|十七|十八|十九|二十"
var ChineseNumberSlice = strings.Split(ChineseNumber, "|")
var MarkRegex = regexp.MustCompile(`[\p{P}\p{S}]`)
var SeasonTitleMatch = regexp.MustCompile(`第(\d{1,2})季`)
var MatchFirstSeason = regexp.MustCompile(`第[一1]季`)
var MatchLanguage = regexp.MustCompile(`(普通话|粤配|中配|中文|英文|粤语)版`)
var MatchKeyword = regexp.MustCompile(`<em class="keyword">(.*?)</em>`)

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

type manager struct {
	scrapers     []Scraper
	searchers    []MediaSearcher
	initializers []Initializer
}

var adapter = &manager{
	scrapers:     []Scraper{},
	searchers:    []MediaSearcher{},
	initializers: []Initializer{},
}

func GetScraper(platform string) Scraper {
	for _, v := range adapter.scrapers {
		if string(v.Platform()) == platform {
			return v
		}
	}
	return nil
}

func GetSearcher(platform string) MediaSearcher {
	for _, v := range adapter.searchers {
		if string(v.SearcherType()) == platform {
			return v
		}
	}
	return nil
}

func GetInitializers() []Initializer {
	return adapter.initializers
}

func GetPlatforms() []string {
	var result []string
	for _, v := range adapter.scrapers {
		result = append(result, string(v.Platform()))
	}
	return result
}

func Register(i interface{}) {
	if v, ok := i.(MediaSearcher); ok {
		adapter.searchers = append(adapter.searchers, v)
	}
	if v, ok := i.(Scraper); ok {
		adapter.scrapers = append(adapter.scrapers, v)
	}
	if v, ok := i.(Initializer); ok {
		adapter.initializers = append(adapter.initializers, v)
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
