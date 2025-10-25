package danmaku

import (
	"danmu-tool/internal/utils"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// TODO 先使用内存缓存进行操作

// 25（2位） + animeid（6位）+ 源顺序（2位）+ 集编号（4位）

type episodeCache struct {
	season   []string // platform_ss 作key
	platform []Platform
	episode  []string // platform_ss_ep 作key
}

var episodeIdCache = episodeCache{
	platform: []Platform{Bilibili, Tencent},
	season:   make([]string, 0),
	episode:  make([]string, 0),
}
var episodeIdCacheLock = sync.Mutex{}

func DecodeEpisodeId(id int64) string {
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

	episodeIdCacheLock.Lock()
	defer episodeIdCacheLock.Unlock()

	if int(ss-1) >= len(episodeIdCache.season) {
		return ""
	}
	if int(platform-1) >= len(episodeIdCache.platform) {
		return ""
	}
	if int(ep-1) >= len(episodeIdCache.episode) {
		return ""
	}

	return episodeIdCache.episode[ep-1]
}

func GenEpisodeId(p Platform, ss string, ep string) int64 {
	episodeIdCacheLock.Lock()
	defer episodeIdCacheLock.Unlock()

	ep = string(p) + "_" + ss + "_" + ep
	ss = string(p) + "_" + ss

	var seasonId int
	var seasonE bool
	for i, v := range episodeIdCache.season {
		if v == ss {
			seasonId = i + 1
			seasonE = true
			break
		}
	}
	if !seasonE {
		episodeIdCache.season = append(episodeIdCache.season, ss)
		seasonId = len(episodeIdCache.season)
	}

	var pId int
	var pE bool
	for i, v := range episodeIdCache.platform {
		if v == p {
			pId = i + 1
			pE = true
			break
		}
	}
	if !pE {
		episodeIdCache.platform = append(episodeIdCache.platform, p)
		pId = len(episodeIdCache.platform)
	}

	var epId int
	var epE bool
	for i, v := range episodeIdCache.episode {
		if v == ep {
			epId = i + 1
			epE = true
			break
		}
	}
	if !epE {
		episodeIdCache.episode = append(episodeIdCache.episode, ep)
		epId = len(episodeIdCache.episode)
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

	logger.Debug(fmt.Sprintf("episode id cache %v", episodeIdCache))

	return result
}
