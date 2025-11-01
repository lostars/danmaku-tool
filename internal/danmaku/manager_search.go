package danmaku

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/utils"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var EmbySeasonTitleRegex = regexp.MustCompile(`Specials|季\s\d{1,2}|第*\s*\d{1,2}\s*季`)

func MatchMedia(param MatchParam) []*Media {

	// 替换黑名单搜索词 但是用于emby匹配的关键词不做替换
	embyKeyword := param.FileName
	for _, w := range config.GetConfig().Tokenizer.Blacklist {
		if w.Key == param.FileName {
			param.FileName = w.Value
			break
		}
	}
	if config.GetConfig().EmbyEnabled() {
		search, err := SearchEmby(embyKeyword, param.SeasonId)
		if err == nil && search.Items != nil && len(search.Items) > 0 {
			// 默认取第一个
			item := search.Items[0]
			param.Emby.Name = item.Name
			param.Emby.ItemId = item.Id
			param.Emby.Type = item.Type
			if item.Type == "Movie" {
				param.Emby.ProductionYear = item.ProductionYear
			}
		}
		if param.SeasonId >= 0 {
			season, err := GetSeasons(param.Emby.ItemId)
			if err == nil {
				for _, s := range season.Items {
					if s.IndexNumber != param.SeasonId {
						continue
					}
					param.Emby.ProductionYear = s.ProductionYear
					if !EmbySeasonTitleRegex.MatchString(s.Name) {
						if param.SeasonId > 0 {
							param.FileName += s.Name
						}
					} else {
						if param.SeasonId > 1 && param.SeasonId <= len(ChineseNumberSlice) {
							param.FileName += strings.Join([]string{"第", ChineseNumberSlice[param.SeasonId-1], "季"}, "")
						}
					}
					break
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
