package youku

import (
	"danmu-tool/internal/danmaku"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

func (c *client) Match(param danmaku.MatchParam) ([]*danmaku.Media, error) {
	keyword := param.FileName
	ssId := int64(-1)
	var err error
	matches := danmaku.SeriesRegex.FindStringSubmatch(keyword)
	if len(matches) > 3 {
		keyword = matches[1]
		ssId, err = strconv.ParseInt(matches[2], 10, 64)
		if err == nil {
			if ssId > 1 && ssId <= 20 {
				keyword = strings.Join([]string{matches[1], "第", danmaku.ChineseNumberSlice[ssId-1], "季"}, "")
			}
		}
	}

	params := map[string]interface{}{
		"searchType": 1,
		"keyword":    keyword,
		"pg":         1,
		"pz":         30,
		"appCaller":  "pc",
		"appScene":   "mobile_multi",
		// 只搜索 影视 分类
		"categories": "2007",
	}

	urlParams, _ := c.sign(params, search)
	paramsBytes, _ := json.Marshal(params)
	urlParams.Set("data", string(paramsBytes))

	api := fmt.Sprintf("https://acs.youku.com/h5/%s/2.0/?%s", search.api, urlParams.Encode())
	req, _ := http.NewRequest(http.MethodGet, api, nil)
	c.setReq(req)

	resp, err := c.common.DoReq(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResult APIResult
	err = json.NewDecoder(resp.Body).Decode(&apiResult)
	if err != nil {
		return nil, err
	}
	if !apiResult.success() {
		return nil, fmt.Errorf("match request fail: %s", strings.Join(apiResult.Ret, ","))
	}
	if apiResult.Data.Nodes == nil || len(apiResult.Data.Nodes) < 1 {
		return nil, fmt.Errorf("empty nodes: %s", keyword)
	}

	var result []*danmaku.Media
	for _, n := range apiResult.Data.Nodes {
		if n.Nodes == nil || len(n.Nodes) == 0 {
			continue
		}
		// 没有基础信息
		if n.Nodes[0].Nodes == nil || len(n.Nodes[0].Nodes) < 1 {
			continue
		}
		mediaInfo := n.Nodes[0].Nodes[0].Data
		if mediaInfo.IsYouku == 0 || mediaInfo.IsTrailer == 1 {
			continue
		}
		// 过滤标签
		tag := mediaInfo.PosterDTO.IconCorner.TagText
		if blackListRegex.MatchString(tag) {
			continue
		}

		media := &danmaku.Media{
			Id:       mediaInfo.RealShowId,
			Title:    mediaInfo.TempTitle,
			Desc:     mediaInfo.Info,
			TypeDesc: mediaInfo.Cats,
			Platform: danmaku.Youku,
		}

		if len(n.Nodes) == 1 {
			// 电影
			if ssId >= 0 || mediaInfo.EpisodeTotal > 1 {
				continue
			}
			media.Type = danmaku.Movie
			// 获取videoId
			vid := c.getVID(mediaInfo.RealShowId)
			if vid == "" {
				continue
			}
			videoInfo, e := c.videoInfo(vid)
			if e != nil {
				continue
			}
			var eps = make([]*danmaku.MediaEpisode, 0, 1)
			eps = append(eps, &danmaku.MediaEpisode{
				Id:        vid,
				Title:     videoInfo.Title,
				EpisodeId: videoInfo.Title,
			})
			media.Episodes = eps

		} else {
			// 剧集
			if ssId < 0 || mediaInfo.EpisodeTotal <= 1 {
				continue
			}
			media.Type = danmaku.Series
			var eps = make([]*danmaku.MediaEpisode, 0, len(n.Nodes[1].Nodes))
			for _, epInfo := range n.Nodes[1].Nodes {
				eps = append(eps, &danmaku.MediaEpisode{
					Id:        epInfo.Data.VideoId,
					Title:     epInfo.Data.Title,
					EpisodeId: epInfo.Data.ShowVideoStage,
				})
			}
			media.Episodes = eps
		}

		result = append(result, media)
	}

	return result, nil
}

func (c *client) getVID(showId string) string {
	//	https://v.youku.com/video?s=ecba3364afbe46aaa122 会 302 到视频地址
	req, _ := http.NewRequest(http.MethodGet, "https://v.youku.com/video?s="+showId, nil)
	c.common.HttpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := c.common.DoReq(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	location := resp.Header.Get("Location")
	// /v_show/id_XNjM2OTM4MjY0NA==.html?s=ecba3364afbe46aaa122
	matches := matchVIDRegex.FindStringSubmatch(location)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

var matchVIDRegex = regexp.MustCompile(`/v_show/id_([a-zA-Z0-9=]+)\.html`)

func (c *client) GetDanmaku(id string) ([]*danmaku.StandardDanmaku, error) {
	// [platform]_[id]_[id]
	s := strings.Split(id, "_")
	if len(s) != 3 {
		return nil, danmaku.PlatformError(danmaku.Youku, "invalid id")
	}

	info, err := c.videoInfo(s[2])
	if err != nil {
		c.common.Logger.Error(fmt.Sprintf("%s video info error", err.Error()))
		return nil, err
	}

	duration, err := strconv.ParseFloat(info.Seconds, 64)
	if err != nil {
		return nil, err
	}

	return c.scrapeDanmaku(s[2], int(duration/60+1)), nil
}

var blackListRegex = regexp.MustCompile(`短剧`)

func (c *client) SearcherType() danmaku.Platform {
	return danmaku.Youku
}
