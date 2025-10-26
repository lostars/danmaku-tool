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
	episodeId -> memory_cache -> [platform]_[id]_[id] -> platform scraper

	最终用于获取弹幕的都是最后的 拼接 字符串，用于多平台，同时抓取弹幕的保存结构也是如此。
	方便后续服务以无状态运行。

	唯一的问题就是每次重启服务会导致同一集的 episodeId 发生变化，因为这个完全是按照请求的次序来编码id的。
	同时依赖于api /match /comment/{id} 的成对调用，match命中后缓存映射关系，然后返回映射好的 episodeId。
	有些前端js插件会缓存 episodeId，服务重启后映射关系丢失，导致查询不到弹幕。

	memory_cache 指的是 episodeId 和 实际剧集信息的映射关系，并不是指缓存弹幕数据或者剧集信息本身。
*/

func (c *realTimeData) Search(param MatchParam) (*MatchResult, error) {

	strs := strings.Split(param.FileName, " ")
	if strs[0] == "" {
		return nil, errors.New("invalid param")
	}
	searchTitle := strs[0]
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

	for _, s := range danmaku.ManagerOfDanmaku.Searchers {
		media, err := s.Search(searchTitle)
		if err != nil {
			logger.Error(err.Error(), "searchType", s.SearcherType(), "title", searchTitle)
			continue
		}
		logger.Debug("search success", "searchType", s.SearcherType(), "title", searchTitle)
		for _, m := range media {
			if m.Episodes == nil || len(m.Episodes) == 0 {
				continue
			}
			if searchMovies {
				result.IsMatched = true
				result.Matches = append(result.Matches, Match{
					EpisodeId:    c.genEpisodeId(s.SearcherType(), m.Id, m.Episodes[0].Id),
					AnimeTitle:   m.Title,
					EpisodeTitle: m.Episodes[0].Title,
				})
			} else {
				for _, ep := range m.Episodes {
					epStr := strconv.FormatInt(epId, 10)
					if ep.EpisodeId == epStr {
						logger.Info("ep match success", "searchType", s.SearcherType(), "title", searchTitle, "ep", ep.EpisodeId)
						result.IsMatched = true
						result.Matches = append(result.Matches, Match{
							EpisodeId:    c.genEpisodeId(s.SearcherType(), m.Id, ep.Id),
							AnimeTitle:   m.Title,
							EpisodeTitle: ep.EpisodeId,
						})
						break
					}
				}
			}
		}
	}

	return result, nil
}

func (c *realTimeData) GetDanmaku(param CommentParam) (*CommentResult, error) {
	id := c.decodeEpisodeId(param.Id)

	ids := strings.Split(id, "_")
	if len(ids) != 3 {
		return nil, errors.New("invalid param")
	}
	var scraper = danmaku.ManagerOfDanmaku.Searchers[ids[0]]
	if scraper == nil {
		return nil, errors.New("invalid param")
	}
	data, err := scraper.GetDanmaku(id)
	if err != nil {
		return nil, err
	}

	// merge danmaku
	var source []*danmaku.StandardDanmaku
	if config.GetConfig().Bilibili.MergeDanmakuInMills > 0 {
		var merged = danmaku.MergeDanmaku(data, config.GetConfig().Bilibili.MergeDanmakuInMills, 0)
		source = make([]*danmaku.StandardDanmaku, 0, len(merged))
		for _, v := range merged {
			source = append(source, v)
		}
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

// 25（2位） + animeid（6位）+ 源顺序（2位）+ 集编号（4位）

type realTimeData struct {
	season   []string // platform_ss 作key
	platform []string
	episode  []string // platform_ss_ep 作key
	lock     sync.Mutex
}

func (c *realTimeData) decodeEpisodeId(id int64) string {
	str := strconv.FormatInt(id, 10)
	if len(str) != 14 {
		return ""
	}
	ss, err := strconv.ParseInt(str[2:8], 10, 64)
	if err != nil {
		return ""
	}
	platform, err := strconv.ParseInt(str[8:10], 10, 64)
	if err != nil {
		return ""
	}
	ep, err := strconv.ParseInt(str[10:14], 10, 64)
	if err != nil {
		return ""
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if int(ss-1) >= len(c.season) {
		return ""
	}
	if int(platform-1) >= len(c.platform) {
		return ""
	}
	if int(ep-1) >= len(c.episode) {
		return ""
	}

	return c.episode[ep-1]
}

func (c *realTimeData) genEpisodeId(p danmaku.Platform, ss string, ep string) int64 {
	c.lock.Lock()
	defer c.lock.Unlock()

	ep = string(p) + "_" + ss + "_" + ep
	ss = string(p) + "_" + ss

	var seasonId int
	var seasonE bool
	for i, v := range c.season {
		if v == ss {
			seasonId = i + 1
			seasonE = true
			break
		}
	}
	if !seasonE {
		c.season = append(c.season, ss)
		seasonId = len(c.season)
	}

	var pId int
	var pE bool
	for i, v := range c.platform {
		if v == string(p) {
			pId = i + 1
			pE = true
			break
		}
	}
	if !pE {
		c.platform = append(c.platform, string(p))
		pId = len(c.platform)
	}

	var epId int
	var epE bool
	for i, v := range c.episode {
		if v == ep {
			epId = i + 1
			epE = true
			break
		}
	}
	if !epE {
		c.episode = append(c.episode, ep)
		epId = len(c.episode)
	}

	var season = fmt.Sprintf("%0*d", 6, seasonId)
	var platform = fmt.Sprintf("%0*d", 2, pId)
	var episode = fmt.Sprintf("%0*d", 4, epId)

	idStr := strings.Join([]string{"25", season, platform, episode}, "")
	result, err := strconv.ParseInt(idStr, 10, 64)
	logger := utils.GetComponentLogger("manager")
	if err != nil {
		logger.Error("gen episode id err", "id", idStr)
		return 0
	}

	logger.Debug(fmt.Sprintf("episode id cache %v", c))

	return result
}
