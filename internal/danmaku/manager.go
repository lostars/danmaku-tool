package danmaku

import (
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

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

type PlatformClient struct {
	MaxWorker  int
	Cookie     string
	HttpClient *http.Client

	XmlPersist *DataXMLPersist
	Logger     *slog.Logger
}

type Scraper interface {
	PlatformInitializer
	// Scrape 抓取并保存弹幕 各个平台视频id/剧集id 看各自实现
	Scrape(id string) error
	// GetDanmaku 实时获取平台弹幕 id是各自平台的视频id
	GetDanmaku(id string) ([]*StandardDanmaku, error)
	// Match 匹配剧集信息，如果是剧集，会获取ep信息同时返回
	Match(param MatchParam) ([]*Media, error)
}

type PlatformInitializer interface {
	Init() error
	Platform() Platform
}

var SeriesRegex = regexp.MustCompile("(.*)\\sS(\\d{1,3})E(\\d{1,3})$")
var ChineseNumber = "一|二|三|四|五|六|七|八|九|十|十一|十二|十三|十四|十五|十六|十七|十八|十九|二十"
var ChineseNumberSlice = strings.Split(ChineseNumber, "|")
var MarkRegex = regexp.MustCompile(`[\p{P}\p{S}]`)
var SeasonTitleMatch = regexp.MustCompile(`第(\d{1,2})季`)
var MatchFirstSeason = regexp.MustCompile(`第[一1]季`)
var MatchLanguage = regexp.MustCompile(`(日语|普通话|粤配|中配|中文|英文|粤语)版`)
var MatchKeyword = regexp.MustCompile(`<em class="keyword">(.*?)</em>`)

// https://api.dandanplay.net/swagger/index.html#/%E5%BC%B9%E5%B9%95/Comment_GetComment
// p 出现时间,模式,颜色,用户ID

type StandardDanmaku struct {
	OffsetMills int64 // 偏移量 ms 注意dandan中保存的是秒，保留2位小数，这里为了精度使用ms，在API返回或者写入时才进行转换
	Mode        int   // 1滚动 4底部 5顶部
	Color       int   // 颜色 数字格式 16777215
	// 以上三个字段按照顺序兼容dandan API p字段

	Content string // dandan API m字段

	// 以下字段用于其他记录
	FontSize int32 // 字体大小
	Platform Platform
}

type MatchParam struct {
	FileName            string `json:"fileName"`
	FileSize            int64  `json:"fileSize"`
	MatchMod            string `json:"matchMod"` // fileNameOnly
	DurationSeconds     int64  `json:"videoDuration"`
	FileHash            string `json:"fileHash"`
	SeasonId, EpisodeId int
	// Emby 内部搜索参数 反查Emby用于更加精准的搜索
	Emby struct {
		// 年份数字（2025） 匹配时 判断年份是否在年份闭区间内
		// 电影开始结束将会一样，剧集则会根据剧集状态修改结束时间，如果一直更新则会将结束年份设置为一个很大的值保证匹配
		ProductionYear, ProductionYearEnd int
		// 剧集或者电影名称 这个和dandan api搜索的应该一致
		Name string
		// 类型: "Movie" "Series"
		Type string
		// emby 内部 id 503357 保留
		ItemId string
	}
}

func (p MatchParam) MatchYear(year int) bool {
	if p.Emby.ProductionYear > 0 && p.Emby.ProductionYearEnd > 0 {
		return year <= p.Emby.ProductionYearEnd && year >= p.Emby.ProductionYear
	}
	return true
}

const WhiteColor = 16777215

const RollMode = 1
const BottomMode = 4
const TopMode = 5

type manager struct {
	scrapers     []Scraper
	initializers []PlatformInitializer
}

var adapter = &manager{
	scrapers:     []Scraper{},
	initializers: []PlatformInitializer{},
}

func GetScraper(platform string) Scraper {
	for _, v := range adapter.scrapers {
		if string(v.Platform()) == platform {
			return v
		}
	}
	return nil
}

func GetInitializers() []PlatformInitializer {
	return adapter.initializers
}

func GetPlatforms() []string {
	var result []string
	for _, v := range adapter.scrapers {
		result = append(result, string(v.Platform()))
	}
	return result
}

func RegisterScraper(s Scraper) {
	adapter.scrapers = append(adapter.scrapers, s)
}

func RegisterInitializer(i PlatformInitializer) {
	adapter.initializers = append(adapter.initializers, i)
}

type Platform string

const (
	Bilibili = "bilibili"
	Tencent  = "tencent"
	Youku    = "youku"
	Iqiyi    = "iqiyi"
)

type DataPersistType string

const (
	XMLPersistType = "xml"
)
