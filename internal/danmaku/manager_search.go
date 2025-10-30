package danmaku

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/utils"
	"sort"
	"sync"
)

func MatchMedia(param MatchParam) []*Media {

	searchers := adapter.searchers

	logger := utils.GetComponentLogger("search-media")

	var result []*Media

	lock := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(searchers))
	for _, s := range searchers {
		go func(searcher MediaSearcher) {
			defer wg.Done()
			media, err := searcher.Match(param)
			if err != nil {
				logger.Error(err.Error(), "searchType", searcher.SearcherType(), "title", param.FileName)
				return
			}
			logger.Debug("search success", "size", len(media), "searchType", searcher.SearcherType(), "title", param.FileName)

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
