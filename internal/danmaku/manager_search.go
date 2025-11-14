package danmaku

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/utils"
	"fmt"
	"sort"
	"sync"
	"time"
)

const searchMediaC = "search_media"

func MatchMedia(param MatchParam) []*Media {
	// 如果未设置季信息，则从标题中解析
	if param.SeasonId < 0 {
		param.SeasonId = MatchSeason(param.Title)
	}
	// 预处理标题
	param.Title = ClearTitleAndSeason(param.Title)
	// 从emby获取年份等信息
	if config.EmbyEnabled() {
		search, err := SearchEmby(param.Title, param.SeasonId)
		if err == nil && len(search.Items) > 0 {
			if len(search.Items) > 1 {
				utils.WarnLog(searchMediaC, fmt.Sprintf("[%s] match more than 1 emby media", param.Title))
			}
			// 默认取第一个
			item := search.Items[0]
			switch item.Type {
			case EmbySeries:
				// 只有多季的剧集才获取单季发布年份
				if season, e := GetSeasons(item.Id, false); e == nil && len(season.Items) > 1 {
					for _, s := range season.Items {
						if s.IndexNumber == param.SeasonId {
							param.ProductionYear = s.ProductionYear
							break
						}
					}
				}
			case EmbyMovie:
				param.ProductionYear = item.ProductionYear
			}
		}
	}

	var result []*Media
	lock := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(adapter.scrapers))
	for _, s := range adapter.scrapers {
		go func(scraper Scraper) {
			defer wg.Done()
			// 并发 复制参数进行处理
			searchParam := param
			if s.Platform() == Bilibili {
				searchParam.CheckEm = true
			}
			searchParam.Platform = scraper.Platform()

			start := time.Now()
			media, err := scraper.Match(searchParam)
			if err != nil {
				utils.ErrorLog(searchMediaC, err.Error(), "platform", scraper.Platform(), "title", param.Title)
				return
			}
			utils.InfoLog(searchMediaC, fmt.Sprintf("[%s] match done", s.Platform()), "cost_ms", time.Since(start).Milliseconds())
			if len(media) < 1 {
				utils.DebugLog(searchMediaC, fmt.Sprintf("[%s] match no result", s.Platform()))
			}

			lock.Lock()
			result = append(result, media...)
			lock.Unlock()
		}(s)
	}
	wg.Wait()

	// 结果排序
	sort.Slice(result, func(i, j int) bool {
		a := config.GetPlatformConfig(string(result[i].Platform))
		b := config.GetPlatformConfig(string(result[j].Platform))
		return a.Priority < b.Priority
	})

	return result
}
