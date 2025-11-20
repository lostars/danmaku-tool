package danmaku

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/utils"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const searchMediaC = "search_media"

func MatchMedia(param MatchParam) []*Media {
	// 如果未设置季信息，则从标题中解析
	if param.SeasonId < 0 {
		param.SeasonId = MatchSeason(param.Title)
	}
	matchYear := true
	for _, y := range config.GetConfig().Tokenizer.YearMatchList {
		// 如果手动设置了年份匹配 则跳过从元数据获取年份
		if y.Title == param.Title {
			match := false
			if y.Season != nil && *y.Season >= 0 {
				if param.SeasonId == *y.Season {
					match = true
				}
			} else {
				match = true
			}
			if match {
				utils.InfoLog(searchMediaC, fmt.Sprintf("%s matched year-match-list", param.Title), "year", y.Year, "season", y.Season)
				matchYear = false
				param.ProductionYear = y.Year
				break
			}
		}
	}
	// 预处理标题
	param.Title = ClearTitleAndSeason(param.Title)
	// 从元数据提供商获取年份信息
	if matchYear {
		for _, meta := range adapter.metadata {
			year, err := meta.Year(param.Title, strconv.FormatInt(int64(param.SeasonId), 10))
			if err != nil {
				utils.ErrorLog(searchMediaC, err.Error())
			}
			if year > 0 {
				param.ProductionYear = year
				utils.InfoLog(searchMediaC, fmt.Sprintf("%s matched year", param.Title), "year", year, "source", meta.Source())
				// 匹配到一个即可
				break
			}
		}
	}

	ch := make(chan []*Media, len(adapter.scrapers))
	wg := sync.WaitGroup{}
	wg.Add(len(adapter.scrapers))
	for _, s := range adapter.scrapers {
		go func(scraper Scraper) {
			defer wg.Done()
			// 并发 复制参数进行处理
			searchParam := param
			searchParam.CheckEm = s.CheckEm()
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
			ch <- media
		}(s)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	var result = make([]*Media, 0, 100)
	for m := range ch {
		for _, media := range m {
			// 过滤掉没有ep的剧集
			if len(media.Episodes) < 1 && media.EpisodeCount <= 0 {
				continue
			}
			// 剧集匹配规则
			if param.SeasonId >= 0 {
				rematch(media, param)
			}
			result = append(result, media)
		}
	}

	// 结果排序
	sort.Slice(result, func(i, j int) bool {
		a := config.GetPlatformConfig(string(result[i].Platform))
		b := config.GetPlatformConfig(string(result[j].Platform))
		return a.Priority < b.Priority
	})

	return result
}

func rematch(media *Media, param MatchParam) {
	for _, re := range config.GetConfig().Tokenizer.Rematch {
		if re.Platform != string(media.Platform) {
			continue
		}
		if re.MediaId != media.Id {
			scraper := GetScraper(string(media.Platform))
			// 支持iqiyi字符串id配置
			if parser, ok := scraper.(MediaIdParser); ok {
				number := parser.ParseNumber(re.MediaId)
				if number <= 0 || media.Id != strconv.FormatInt(number, 10) {
					continue
				}
			} else {
				continue
			}
		}
		if len(re.Targets) > 0 {
			targetMatch(media, re, param)
			return
		}

		for _, epMatch := range re.Episodes {
			if epMatch.Season != param.SeasonId {
				continue
			}
			matches := strings.Split(epMatch.Episode, ",")
			if len(matches) < 1 {
				return
			}

			var eps = make([]*MediaEpisode, 0, len(media.Episodes))
			index := int64(1)
			for _, match := range matches {
				start, end, ok := rangeMatch(match)
				for i, ep := range media.Episodes {
					epId, err := strconv.ParseInt(ep.EpisodeId, 10, 64)
					if err != nil {
						epId = int64(i + 1)
					}
					if ok {
						if epId >= start && epId <= end {
							eps = append(eps, &MediaEpisode{
								Id:        ep.Id,
								EpisodeId: strconv.FormatInt(index, 10),
								Title:     ep.Title,
							})
							index++
						}
					} else {
						if strconv.FormatInt(epId, 10) == match {
							eps = append(eps, &MediaEpisode{
								Id:        ep.Id,
								EpisodeId: strconv.FormatInt(index, 10),
								Title:     ep.Title,
							})
							index++
							break
						}
					}
				}
			}
			media.Episodes = eps
			// 匹配到搜索的季即可
			return
		}
	}
}

func targetMatch(media *Media, re config.Rematch, param MatchParam) {

	itemData := make([]map[int]int64, 0, len(re.Targets))
	target := -1
	union := false
	for i, item := range re.Targets {
		metadata := GetMetadata(item.Source)
		if metadata == nil {
			utils.ErrorLog(searchMediaC, fmt.Sprintf("[%s] source not found", item.Source))
			continue
		}
		if eps, e := metadata.Episodes(item.Item); e == nil && len(eps) > 0 {
			itemMap := map[int]int64{}
			for _, s := range eps {
				if target < 0 && ClearTitleAndSeason(s.Title) == param.Title {
					target = i
					union = item.Union
				}
				itemMap[s.SeasonId] = itemMap[s.SeasonId] + 1
			}
			itemData = append(itemData, itemMap)
		}
	}
	if target < 0 {
		return
	}

	var start, end int64
	preCount := int64(0)
	sort.Slice(itemData, func(i, j int) bool {
		return i < j
	})
outer:
	for i, d := range itemData {
		var seasons = make([]int64, len(d)+1)
		for k, v := range d {
			seasons[k] = v
		}
		if i == target {
			for s, v := range seasons {
				if param.SeasonId == s {
					end = start + v
					start++
					break outer
				} else {
					start += v
					preCount += v
				}
			}
		} else {
			for _, v := range seasons {
				start += v
			}
		}
	}
	if end <= 0 {
		return
	}
	var eps = make([]*MediaEpisode, 0, end-start+1)
	index := preCount + 1
	preIndex := int64(0)
	for _, ep := range media.Episodes {
		epId, err := strconv.ParseInt(ep.EpisodeId, 10, 64)
		if err != nil {
			return
		}
		if epId >= start && epId <= end {
			id := epId
			if union {
				// 处理漏集
				if preIndex > 0 && epId-preIndex > 1 {
					id = index + epId - preIndex
				} else {
					id = index
				}
				index++
				preIndex = epId
			}
			eps = append(eps, &MediaEpisode{
				Id:        ep.Id,
				EpisodeId: strconv.FormatInt(id, 10),
				Title:     ep.Title,
			})
		}
	}
	msg := fmt.Sprintf("targets match %s %s", media.Title, media.Id)
	utils.DebugLog(searchMediaC, msg, "start", start, "end", end, "index", index, "target", target)
	media.Episodes = eps
}

func rangeMatch(match string) (int64, int64, bool) {
	ranges := strings.Split(match, "-")
	if len(ranges) != 2 {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(ranges[0], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	end, err := strconv.ParseInt(ranges[1], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return start, end, true
}
