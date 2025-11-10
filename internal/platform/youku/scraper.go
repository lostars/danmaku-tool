package youku

import (
	"danmaku-tool/internal/danmaku"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (c *client) Scrape(id string) error {

	c.scrapeVideo(id)

	return nil
}

func (c *client) Match(param danmaku.MatchParam) ([]*danmaku.Media, error) {
	keyword := param.Title

	params := map[string]interface{}{
		"searchType": 1,
		"keyword":    keyword,
		"pg":         1,
		"pz":         30,
		"appCaller":  "pc",
		"appScene":   "mobile_multi",
		// 只搜索 影视 分类
		"categories": "2007",
		// 重要 不同版本返回了不同字段 注意调试时候和浏览器环境保持一致
		"sdkver":       313,
		"pcKuFlixMode": 1,
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
		if mediaInfo.IsYouku != 1 || mediaInfo.IsTrailer == 1 {
			continue
		}
		// 过滤标签
		tag := mediaInfo.PosterDTO.IconCorner.TagText
		if blacklistRegex.MatchString(tag) {
			continue
		}
		// 过滤黑名单分类
		if blacklistCatsRegex.MatchString(mediaInfo.Cats) {
			continue
		}
		// 过滤年份
		yearMatches := yearMatchRegex.FindStringSubmatch(mediaInfo.FeatureDTO.Text)
		if len(yearMatches) < 2 {
			continue
		}
		year, _ := strconv.ParseInt(yearMatches[1], 10, 64)
		if !param.MatchYear(int(year)) {
			continue
		}
		match := param.MatchTitle(mediaInfo.TempTitle)
		c.common.Logger.Debug(fmt.Sprintf("[%s] match [%s]: %v", mediaInfo.TempTitle, param.Title, match))
		if !match {
			continue
		}

		media := &danmaku.Media{
			Id:       mediaInfo.RealShowId,
			Title:    mediaInfo.TempTitle,
			Desc:     mediaInfo.Info,
			TypeDesc: mediaInfo.Cats,
			Cover:    mediaInfo.ThumbUrl,
			Platform: danmaku.Youku,
		}

		if mediaInfo.Cats == "电影" {
			// 电影
			media.Type = danmaku.Movie
			// 获取videoId
			vid := c.getVID(mediaInfo.RealShowId)
			if vid == "" {
				continue
			}
			videoInfo, _, e := c.videoInfo(vid)
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
			data := n.Nodes[0]
			if len(n.Nodes) > 1 {
				data = n.Nodes[1]
			}

			media.Type = danmaku.Series
			var eps = make([]*danmaku.MediaEpisode, 0, len(data.Nodes))
			for i, epInfo := range data.Nodes {
				episodeId := epInfo.Data.ShowVideoStage
				if episodeId == "" {
					episodeId = strconv.FormatInt(int64(i+1), 10)
				}
				eps = append(eps, &danmaku.MediaEpisode{
					Id:        epInfo.Data.VideoId,
					Title:     epInfo.Data.Title,
					EpisodeId: episodeId,
				})
			}
			media.Episodes = eps
		}

		result = append(result, media)
	}

	return result, nil
}

func (c *client) GetDanmaku(id string) ([]*danmaku.StandardDanmaku, error) {

	info, _, err := c.videoInfo(id)
	if err != nil {
		c.common.Logger.Error(fmt.Sprintf("%s video info error", err.Error()))
		return nil, err
	}

	duration, err := strconv.ParseFloat(info.Seconds, 64)
	if err != nil {
		return nil, err
	}

	return c.scrapeDanmaku(id, int(duration/60+1)), nil
}
