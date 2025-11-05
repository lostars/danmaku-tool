package service

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
	"encoding/gob"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

/*
	dandan api 实时模式，将dandan api的 episodeId 通过规则映射在内存中
	episodeId -> memory_cache -> [platform]\x00[id]\x00[id] -> platform scraper

	最终用于获取弹幕的都是各平台视频id字符串，方便后续服务以无状态运行。

	唯一的问题就是每次重启服务会导致同一集的 episodeId 发生变化，因为这个完全是按照请求的次序来编码id的。
	同时依赖于api /match /comment/{id} 的成对调用，match命中后缓存映射关系，然后返回映射好的 episodeId。
	有些前端js插件会缓存 episodeId，服务重启后映射关系丢失，导致查询不到弹幕。

	memory_cache 指的是 episodeId 和 实际剧集信息的映射关系，并不是指缓存弹幕数据或者剧集信息本身。
*/

var (
	fileName = "data.gob"
)

func (c *realTimeData) Finalize() error {
	c.lock.RLock()
	defer c.lock.RUnlock()

	p := path.Join(strings.ReplaceAll(config.ConfPath, path.Base(config.ConfPath), ""), fileName)
	file, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("failed to create data file: %w", err)
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)

	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to encode data: %w", err)
	}

	c.logger.Info("save map info to file success")

	return nil
}

func (c *realTimeData) Load() (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	p := path.Join(strings.ReplaceAll(config.ConfPath, path.Base(config.ConfPath), ""), fileName)
	file, err := os.Open(p)
	if err != nil {
		c.ForwardMap = make(map[string]int64, 1000)
		c.ReverseMap = make(map[int64]string, 1000)
		c.IdAllocator = int64(1)
		return false, err
	}
	defer file.Close()

	if err := gob.NewDecoder(file).Decode(c); err != nil {
		return false, fmt.Errorf("failed to decode data: %w", err)
	}

	return true, nil
}

func (c *realTimeData) ServerInit() error {
	c.logger = utils.GetComponentLogger("real_time_service")
	success, err := c.Load()
	if err != nil {
		return err
	}
	if success {
		c.logger.Info("restore data from file success")
	}
	return nil
}

func (c *realTimeData) Match(param MatchParam) (*DanDanResult, error) {
	matches := danmaku.SeriesRegex.FindStringSubmatch(param.FileName)
	epId := int64(-1)
	searchMovies := true
	// 兼容搜索标题为 "xxxx S01E01" 格式
	// 如果无法匹配则默认匹配电影
	searchParam := danmaku.MatchParam{
		DurationSeconds: param.DurationSeconds,
		SeasonId:        -1,
		// match接口用等于判断，防止匹配出错误弹幕
		Mode: danmaku.Equals,
	}
	if len(matches) > 3 {
		ssId, _ := strconv.ParseInt(matches[2], 10, 64)
		epId, _ = strconv.ParseInt(matches[3], 10, 64)
		searchParam.Title = matches[1]
		searchParam.SeasonId = int(ssId)
		searchParam.EpisodeId = int(epId)
		searchMovies = false
	}

	var result = &DanDanResult{
		Matches: make([]Match, 0, 10),
		Success: true,
	}

	media := danmaku.MatchMedia(searchParam)
	// 客户端只会使用第一个结果 但依旧匹配所有搜索结果用于接口调试
	for _, m := range media {
		if m.Episodes == nil || len(m.Episodes) == 0 {
			continue
		}
		if searchMovies {
			result.IsMatched = true
			result.Matches = append(result.Matches, Match{
				EpisodeId:    c.getGlobalID(string(m.Platform), m.Id, m.Episodes[0].Id),
				AnimeTitle:   m.Title + " [" + string(m.Platform) + "]",
				EpisodeTitle: m.Episodes[0].Title,
			})
			c.logger.Info("movie match success", "platform", m.Platform, "title", param.FileName)
		} else {
			for _, ep := range m.Episodes {
				epStr := strconv.FormatInt(epId, 10)
				if ep.EpisodeId == epStr {
					c.logger.Info("ep match success", "platform", m.Platform, "title", param.FileName, "ep", ep.EpisodeId)
					result.IsMatched = true
					result.Matches = append(result.Matches, Match{
						EpisodeId:    c.getGlobalID(string(m.Platform), m.Id, ep.Id),
						AnimeTitle:   m.Title + " [" + string(m.Platform) + "]",
						EpisodeTitle: ep.EpisodeId,
					})
				}
			}
		}
	}

	return result, nil
}

func (c *realTimeData) GetDanmaku(param CommentParam) (*CommentResult, error) {
	platform, epId, found := c.decodeGlobalID(param.Id)
	if !found {
		return nil, fmt.Errorf("invalid param")
	}
	var scraper = danmaku.GetScraper(platform)
	if scraper == nil {
		return nil, errors.New("invalid param")
	}
	data, err := scraper.GetDanmaku(epId)
	if err != nil {
		return nil, err
	}

	// merge danmaku
	mergeMills := config.GetConfig().GetPlatformConfig(string(scraper.Platform())).MergeDanmakuInMills
	if mergeMills > 0 {
		data = danmaku.MergeDanmaku(data, mergeMills, 0)
	}

	comment := &CommentResult{
		Count:    int64(len(data)),
		Comments: make([]*Comment, 0, len(data)),
	}

	for _, d := range data {
		comment.Comments = append(comment.Comments, &Comment{
			CID: time.Now().Unix(),
			M:   d.Content,
			P:   d.GenDandanAttribute(),
		})
	}
	return comment, nil
}

func (c *realTimeData) Mode() Mode {
	return realTime
}

type realTimeData struct {
	ForwardMap  map[string]int64
	ReverseMap  map[int64]string
	IdAllocator int64
	lock        sync.RWMutex
	logger      *slog.Logger
}

func combineKey(platform, ssID, epID string) string {
	// ASCII 0
	return platform + "\x00" + ssID + "\x00" + epID
}

func (c *realTimeData) getGlobalID(platform, ssID, epID string) int64 {
	key := combineKey(platform, ssID, epID)

	// 使用读锁快速检查是否已存在
	c.lock.RLock()
	if id, ok := c.ForwardMap[key]; ok {
		c.lock.RUnlock()
		return id
	}
	c.lock.RUnlock()

	c.lock.Lock()
	defer c.lock.Unlock()

	if id, ok := c.ForwardMap[key]; ok {
		return id
	}

	newID := c.IdAllocator
	c.IdAllocator++

	c.ForwardMap[key] = newID
	c.ReverseMap[newID] = key

	if newID >= c.IdAllocator {
		c.IdAllocator = newID + 1
	}

	return newID
}

func (c *realTimeData) decodeGlobalID(globalID int64) (platform string, vid string, found bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	key, found := c.ReverseMap[globalID]
	if !found {
		return "", "", false
	}
	parts := strings.Split(key, "\x00")
	if len(parts) == 3 {
		return parts[0], parts[2], true
	}
	return "", "", false
}
