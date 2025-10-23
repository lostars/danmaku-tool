package danmaku

import (
	"danmu-tool/internal/config"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

type Media struct {
}

type Platform interface {
	Platform() string
	Scrape(id interface{}) error
}

type MediaSearcher interface {
	Search(keyword string) ([]Media, error)
	Id(keyword string) ([]interface{}, error)
	Searcher() string
}

type DataPersist interface {
	WriteToFile(fullPath, filename string) error
	Type() string
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
	getManagerDebugger().Printf("danmaku size before merge: %v\n", len(dms))
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
	getManagerDebugger().Printf("damaku size after merge: %v\n", len(result))
	getManagerDebugger().Printf("danmaku merge cost: %vns\n", end-start)

	return result
}

var debugger sync.Map
var dataDebugger sync.Map
var managerDebugger *log.Logger

func getManagerDebugger() *log.Logger {
	if managerDebugger != nil {
		return managerDebugger
	}
	var logger *log.Logger
	var prefix = "[danmaku-manager] "
	if config.Debug {
		logger = log.New(os.Stdout, prefix, 0)
	} else {
		logger = log.New(io.Discard, prefix, 0)
	}
	managerDebugger = logger
	return managerDebugger
}

func DataDebugger(s DataPersist) *log.Logger {
	var prefix = s.Type()
	v, ok := dataDebugger.Load(prefix)
	if ok {
		logger, err := v.(log.Logger)
		if err {
			return &logger
		}
	}
	var logger *log.Logger
	if config.Debug {
		logger = log.New(os.Stdout, fmt.Sprintf("[%s] ", prefix), 0)
	} else {
		logger = log.New(io.Discard, fmt.Sprintf("[%s] ", prefix), 0)
	}
	dataDebugger.Store(prefix, logger)
	return logger
}

func NewDataError(d DataPersist, text string) error {
	return errors.New(fmt.Sprintf("[%s]: %s", d.Type(), text))
}

func Debugger(p Platform) *log.Logger {
	var prefix = p.Platform()
	v, ok := debugger.Load(prefix)
	if ok {
		logger, err := v.(log.Logger)
		if err {
			return &logger
		}
	}
	var logger *log.Logger
	if config.Debug {
		logger = log.New(os.Stdout, fmt.Sprintf("[%s] ", prefix), 0)
	} else {
		logger = log.New(io.Discard, fmt.Sprintf("[%s] ", prefix), 0)
	}
	debugger.Store(prefix, logger)
	return logger
}

func NewError(p Platform, text string) error {
	return errors.New(fmt.Sprintf("[%s] %s", p.Platform(), text))
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
		result = append(result, v.Platform())
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
	e := ManagerOfDanmaku.Platforms[p.Platform()]
	if e != nil {
		return errors.New(fmt.Sprintf("%s registered", p.Platform()))
	}
	ManagerOfDanmaku.Platforms[p.Platform()] = p
	return nil
}
