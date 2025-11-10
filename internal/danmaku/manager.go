package danmaku

import (
	"log/slog"
	"net/http"
	"time"
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
	Cover    string
	Year     int
	PubTime  int64 // unix seconds
	Episodes []*MediaEpisode
	Platform Platform
}

func (m *Media) FormatPubTime(force bool) string {
	var pubTime time.Time
	if m.PubTime > 0 {
		pubTime = time.Unix(m.PubTime, 0)
	} else {
		if m.Year > 0 {
			pubTime = time.Date(m.Year, 1, 1, 0, 0, 0, 0_000_000, time.UTC)
		} else {
			if force {
				pubTime = time.Now()
			} else {
				return ""
			}
		}
	}
	return pubTime.Format(time.RFC3339Nano)
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

	Logger *slog.Logger
}

type Scraper interface {
	Initializer
	// Scrape 抓取并保存弹幕 各个平台视频id/剧集id 看各自实现
	Scrape(id string) error
	// GetDanmaku 实时获取平台弹幕 id是各自平台的视频id
	GetDanmaku(id string) ([]*StandardDanmaku, error)
	// Match 匹配剧集信息，如果是剧集，会获取ep信息同时返回
	Match(param MatchParam) ([]*Media, error)
	Platform() Platform
}

type MediaService interface {
	Media(id string) (*Media, error)
	Scraper
}

type SerializerData struct {
	Platform            Platform
	fullPath, filename  string
	Data                []*StandardDanmaku
	DurationInMills     int64
	SeasonId, EpisodeId string
	// ass 文件用
	ResX, ResY int // 视频分辨率
}
type DataSerializer interface {
	Serialize(data *SerializerData) error
	Type() string
}

const (
	XMLSerializer = "xml"
	ASSSerializer = "ass"
)

type Finalizer interface {
	Finalize() error
}

// ServerInitializer 初始化server需要的操作，实现该接口并注册 RegisterInitializer 即可
type ServerInitializer interface {
	ServerInit() error
}

type Initializer interface {
	Init() error
}

// https://api.dandanplay.net/swagger/index.html#/%E5%BC%B9%E5%B9%95/Comment_GetComment
// p 出现时间,模式,颜色,用户ID

type StandardDanmaku struct {
	OffsetMills int64 // 偏移量 ms 注意dandan中保存的是秒，保留2位小数，这里为了精度使用ms，在API返回或者写入时才进行转换
	Mode        int   // 1普通 4底部 5顶部
	Color       int   // 颜色 数字格式 16777215
	// 以上三个字段按照顺序兼容dandan API p字段

	Content string // dandan API m字段

	// 以下字段用于其他记录
	FontSize int32 // 字体大小
	Platform Platform
}

type MatchParam struct {
	// 视频时长
	DurationSeconds int64
	// 季 集 数字id，默认为-1，代表无季集信息
	SeasonId, EpisodeId int
	// 电影或者剧集数字年份
	ProductionYear int
	// 用于搜索的标题 用于直接搜索无需再次处理
	Title string
	// 匹配模式 MatchMode
	Mode MatchMode
	// 平台
	Platform Platform
	// 是否检查em标签 腾讯和b站返回结果带em标签用于判断是否命中
	CheckEm bool
}

const WhiteColor = 16777215

const NormalMode = 1
const BottomMode = 4
const TopMode = 5

type manager struct {
	scrapers     []Scraper
	initializers []interface{}
	serializers  map[string]DataSerializer
}

var adapter = &manager{
	scrapers:     []Scraper{},
	initializers: []interface{}{},
	serializers:  map[string]DataSerializer{},
}

func GetScraper(platform string) Scraper {
	for _, v := range adapter.scrapers {
		if string(v.Platform()) == platform {
			return v
		}
	}
	return nil
}

func GetMediaService(platform string) MediaService {
	for _, v := range adapter.scrapers {
		if platform == string(v.Platform()) {
			if s, ok := v.(MediaService); ok {
				return s
			}
		}
	}
	return nil
}

func GetInitializers() []interface{} {
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

func RegisterInitializer(i interface{}) {
	adapter.initializers = append(adapter.initializers, i)
}

type Platform string

const (
	Bilibili = "bilibili"
	Tencent  = "tencent"
	Youku    = "youku"
	Iqiyi    = "iqiyi"
)
