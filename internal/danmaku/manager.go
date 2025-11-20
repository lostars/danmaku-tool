package danmaku

import (
	"net/http"

	"github.com/go-co-op/gocron/v2"
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

type MediaEpisode struct {
	Id        string // 存储平台实际的id
	EpisodeId string // 第几话
	Title     string

	SeasonId int
}

type PlatformClient struct {
	MaxWorker  int
	Cookie     string
	HttpClient *http.Client
	Disabled   bool
}

type Scraper interface {
	// Scrape 抓取并保存弹幕 各个平台视频id/剧集id 看各自实现
	Scrape(id string) error
	// GetDanmaku 实时获取平台弹幕 id是各自平台的视频id
	GetDanmaku(id string) ([]*StandardDanmaku, error)
	Media(id string) (*Media, error)
	// Match 匹配剧集信息，如果是剧集，会获取ep信息同时返回
	Match(param MatchParam) ([]*Media, error)
	// CheckEm 是否检查搜索结果em标签
	CheckEm() bool
	Platform() Platform
}

type MediaIdParser interface {
	// ParseNumber 将字符串id转换成数字id，<=0 则失败
	ParseNumber(id string) int64
}

type MetadataService interface {
	Source() Source
	// Episodes 获取所有ep
	Episodes(id string) ([]*MediaEpisode, error)
	// Year 搜索获取媒体年份 如果传入了季id则获取对应季的年份 季节只有多余1季的才获取季节年份
	// 默认只返回第一个搜索结果的年份
	Year(name string, seasonId string) (int, error)
}

type Job interface {
	CreateJob(scheduler gocron.Scheduler) error
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

// ServerInitializer 初始化server操作，实现该接口并注册 Register 即可
type ServerInitializer interface {
	ServerInit() error
}

// AsyncServerInitializer 异步初始化server操作，实现该接口并注册 Register 即可
type AsyncServerInitializer interface {
	ServerInitializer
	AsyncInit() error
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

type InternalMatchParam struct {
	MediaId string
	Title   string
	Year    int
}

const WhiteColor = 16777215

const NormalMode = 1
const BottomMode = 4
const TopMode = 5

type manager struct {
	platforms          []Scraper
	scrapers           []Scraper
	metadata           []MetadataService
	initializers       []Initializer
	serverInitializers []ServerInitializer
	serializers        map[string]DataSerializer
	finalizers         []Finalizer
	jobs               []Job
}

var adapter = &manager{
	serializers: map[string]DataSerializer{},
}

func GetScraper(platform string) Scraper {
	for _, v := range adapter.scrapers {
		if string(v.Platform()) == platform {
			return v
		}
	}
	return nil
}

func GetMetadata(src string) MetadataService {
	for _, v := range adapter.metadata {
		if string(v.Source()) == src {
			return v
		}
	}
	return nil
}

func Initializers() []Initializer {
	return adapter.initializers
}

func Finalizers() []Finalizer {
	return adapter.finalizers
}

func ServerInitializers() []ServerInitializer {
	return adapter.serverInitializers
}

func Jobs() []Job {
	return adapter.jobs
}

func Platforms() []string {
	var platforms []string
	for _, p := range adapter.platforms {
		platforms = append(platforms, string(p.Platform()))
	}
	return platforms
}

func ValidPlatform(platform string) bool {
	for _, p := range adapter.platforms {
		if string(p.Platform()) == platform {
			return true
		}
	}
	return false
}

func RegisterScraper(scraper Scraper) {
	reg(&adapter.scrapers, scraper)
}

func RegisterMetadata(metadata MetadataService) {
	reg(&adapter.metadata, metadata)
}

func Register(i interface{}) {
	reg(&adapter.platforms, i)
	reg(&adapter.jobs, i)
	reg(&adapter.serverInitializers, i)
	reg(&adapter.initializers, i)
	reg(&adapter.finalizers, i)
}

func reg[T any](list *[]T, i interface{}) {
	if v, ok := i.(T); ok {
		*list = append(*list, v)
	}
}

type Platform string

const (
	Bilibili = "bilibili"
	Tencent  = "tencent"
	Youku    = "youku"
	Iqiyi    = "iqiyi"
)

type Source string

const (
	Emby = "emby"
)
