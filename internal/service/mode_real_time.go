package service

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"danmu-tool/internal/utils"
	"errors"
	"fmt"
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

func (c *realTimeData) Match(param danmaku.MatchParam) (*MatchResult, error) {

	strs := strings.Split(param.FileName, " ")
	if strs[0] == "" {
		return nil, errors.New("invalid param")
	}
	// 不能截取 后面包含季信息用于搜索
	searchTitle := param.FileName
	searchMovies := false
	if len(strs) == 1 || strs[1] == "" {
		searchMovies = true
	}
	var epId int64
	logger := utils.GetComponentLogger("real_time_service")
	if len(strs) > 1 && strs[1] != "" {
		sStrs := strings.Split(strs[1], "E")
		if len(sStrs) <= 1 {
			return nil, errors.New("invalid param")
		}
		value, err := strconv.ParseInt(sStrs[1], 10, 64)
		if err != nil {
			return nil, errors.New("invalid param")
		}
		epId = value
	}

	var result = &MatchResult{
		Matches: make([]Match, 0, 10),
		Success: true,
	}

	media := danmaku.MatchMedia(param)
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
		} else {
			for _, ep := range m.Episodes {
				epStr := strconv.FormatInt(epId, 10)
				if ep.EpisodeId == epStr {
					logger.Info("ep match success", "searchType", m.Platform, "title", searchTitle, "ep", ep.EpisodeId)
					result.IsMatched = true
					result.Matches = append(result.Matches, Match{
						EpisodeId:    c.getGlobalID(string(m.Platform), m.Id, ep.Id),
						AnimeTitle:   m.Title + " [" + string(m.Platform) + "]",
						EpisodeTitle: ep.EpisodeId,
					})
					break
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
	var searcher = danmaku.GetSearcher(platform)
	if searcher == nil {
		return nil, errors.New("invalid param")
	}
	data, err := searcher.GetDanmaku(epId)
	if err != nil {
		return nil, err
	}

	// merge danmaku
	mergeMills := config.GetConfig().GetPlatformConfig(string(searcher.SearcherType())).MergeDanmakuInMills
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
	forwardMap  map[string]int64
	reverseMap  map[int64]string
	idAllocator int64
	lock        sync.RWMutex
}

func combineKey(platform, ssID, epID string) string {
	// ASCII 0
	return platform + "\x00" + ssID + "\x00" + epID
}

func (c *realTimeData) getGlobalID(platform, ssID, epID string) int64 {
	key := combineKey(platform, ssID, epID)

	// 使用读锁快速检查是否已存在
	c.lock.RLock()
	if id, ok := c.forwardMap[key]; ok {
		c.lock.RUnlock()
		return id
	}
	c.lock.RUnlock()

	c.lock.Lock()
	defer c.lock.Unlock()

	if id, ok := c.forwardMap[key]; ok {
		return id
	}

	newID := c.idAllocator
	c.idAllocator++

	c.forwardMap[key] = newID
	c.reverseMap[newID] = key

	if newID >= c.idAllocator {
		c.idAllocator = newID + 1
	}

	return newID
}

func (c *realTimeData) decodeGlobalID(globalID int64) (platform string, vid string, found bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	key, found := c.reverseMap[globalID]
	if !found {
		return "", "", false
	}
	parts := strings.Split(key, "\x00")
	if len(parts) == 3 {
		return parts[0], parts[2], true
	}
	return "", "", false
}
