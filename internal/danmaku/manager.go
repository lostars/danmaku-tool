package danmaku

import (
	"danmu-tool/internal/utils"
	"errors"
	"fmt"
	"os"
	"time"
)

var logger = utils.GetComponentLogger("manager")

func PlatformError(p PlatformType, text string) error {
	return fmt.Errorf("[%s] %s", p, text)
}

func DataPersistError(d DataPersistType, text string) error {
	return fmt.Errorf("[%s] %s", d, text)
}

type Media struct {
}

type Platform interface {
	Platform() PlatformType
	Scrape(id interface{}) error
}

type MediaSearcher interface {
	Search(keyword string) ([]Media, error)
	Id(keyword string) ([]interface{}, error)
	Searcher() string
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
	Platform string
	Id       string // 全局id 在使用dandan API时有用
}

const RollMode = 1
const BottomMode = 4
const TopMode = 5

func MergeDanmaku(dms []*StandardDanmaku, mergedInMills int64, durationInMills int64) []*StandardDanmaku {
	var start = time.Now().Nanosecond()
	logger.Debug("danmaku size before merge", "size", len(dms))
	var totalBuckets = durationInMills/mergedInMills + 1
	buckets := make(map[int64]map[string]bool, totalBuckets)
	var result = make([]*StandardDanmaku, 0, len(dms))

	for _, d := range dms {
		bid := d.Offset / mergedInMills // 所属时间桶

		if _, ok := buckets[bid]; !ok {
			// 预估长度
			buckets[bid] = make(map[string]bool, int64(len(dms))/totalBuckets+1)
		}

		// 检查当前桶和前一个桶是否出现过（跨桶重复处理）
		if buckets[bid][d.Content] || buckets[bid-1][d.Content] {
			continue
		}

		result = append(result, d)
		buckets[bid][d.Content] = true
	}

	var end = time.Now().Nanosecond()
	logger.Debug("danmaku size before merge", "size", len(result))
	logger.Debug("danmaku merge cost", "duration", end-start)

	return result
}

type Manager struct {
	Platforms map[string]Platform
	Searchers map[string]MediaSearcher
}

var ManagerOfDanmaku = &Manager{
	Platforms: map[string]Platform{},
	Searchers: map[string]MediaSearcher{},
}

func (m *Manager) GetPlatforms() []string {
	var result []string
	for _, v := range m.Platforms {
		result = append(result, string(v.Platform()))
	}
	return result
}

func checkPersistPath(fullPath, filename string) error {
	if fullPath == "" || filename == "" {
		return errors.New("empty save path or filename")
	}

	// check path
	_, fileStatError := os.Stat(fullPath)
	if fileStatError != nil {
		if os.IsNotExist(fileStatError) {
			mkdirError := os.MkdirAll(fullPath, os.ModePerm)
			if mkdirError != nil {
				return errors.New(fmt.Sprintf("create path %s error: %s", fullPath, mkdirError.Error()))
			}
		} else {
			return errors.New(fmt.Sprintf("create path %s error: %s", fullPath, fileStatError.Error()))
		}
	}
	return nil
}

func RegisterPlatform(p Platform) error {
	e := ManagerOfDanmaku.Platforms[string(p.Platform())]
	if e != nil {
		return errors.New(fmt.Sprintf("%s registered", p.Platform()))
	}
	ManagerOfDanmaku.Platforms[string(p.Platform())] = p
	return nil
}

type PlatformType string

const (
	Bilibili = "bilibili"
	Tencent  = "tencent"
)

type DataPersistType string

const (
	DanDanXMLType = "dandanxml"
)
