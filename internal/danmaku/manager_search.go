package danmaku

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/utils"
	"fmt"
	"sort"
	"sync"
	"time"
)

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
		if err == nil && search.Items != nil && len(search.Items) > 0 {
			// 默认取第一个
			item := search.Items[0]
			switch item.Type {
			case EmbySeries:
				if season, e := GetSeasons(item.Id, false); e == nil {
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

	scrapers := adapter.scrapers

	logger := utils.GetComponentLogger("search-media")

	var result []*Media

	lock := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(scrapers))
	for _, s := range scrapers {
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
				logger.Error(err.Error(), "platform", scraper.Platform(), "title", param.Title)
				return
			}
			logger.Info(fmt.Sprintf("[%s] match done", s.Platform()), "cost_ms", time.Since(start).Milliseconds())
			if len(media) < 1 {
				logger.Debug(fmt.Sprintf("[%s] match no result", s.Platform()))
			}

			lock.Lock()
			result = append(result, media...)
			lock.Unlock()
		}(s)
	}
	wg.Wait()

	// 结果排序
	conf := config.GetConfig()
	sort.Slice(result, func(i, j int) bool {
		a := conf.GetPlatformConfig(string(result[i].Platform))
		b := conf.GetPlatformConfig(string(result[j].Platform))
		return a.Priority < b.Priority
	})

	return result
}
