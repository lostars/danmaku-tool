package danmaku

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/utils"
	"sort"
	"strconv"
	"sync"
)

func MatchMedia(param MatchParam) []*Media {

	if config.GetConfig().EmbyEnabled() {
		search, err := SearchEmby(param.FileName, param.SeasonId)
		if err == nil && search.Items != nil && len(search.Items) > 0 {
			// 默认取第一个
			item := search.Items[0]
			param.Emby.Name = item.Name
			param.Emby.ItemId = item.Id
			param.Emby.Type = item.Type
			param.Emby.ProductionYear = item.ProductionYear
			// 默认开始结束一致
			param.Emby.ProductionYearEnd = item.ProductionYear
			if item.Type == "Series" {
				if item.Status == "Continuing" {
					param.Emby.ProductionYearEnd = 1e6
				}
				matches := embyYearRegex.FindStringSubmatch(item.EndDate)
				if len(matches) > 1 {
					year, _ := strconv.ParseInt(matches[1], 10, 64)
					param.Emby.ProductionYearEnd = int(year)
				}
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
			media, err := scraper.Match(param)
			if err != nil {
				logger.Error(err.Error(), "platform", scraper.Platform(), "title", param.FileName)
				return
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
