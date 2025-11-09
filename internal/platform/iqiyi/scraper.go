package iqiyi

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
)

func (c *client) Match(param danmaku.MatchParam) ([]*danmaku.Media, error) {
	keyword := param.Title
	ssId := int64(param.SeasonId)
	// 为了防止 template == 112 出现（此时Videos无数据），带上季信息进行搜索
	if ssId > 0 && ssId <= int64(len(danmaku.ChineseNumberSlice)) {
		keyword += fmt.Sprintf("第%s季", danmaku.ChineseNumberSlice[ssId-1])
	}

	api := "https://mesh.if.iqiyi.com/portal/lw/search/homePageV3?"
	params := url.Values{
		"key":      {keyword},
		"pageNum":  {"1"},
		"pageSize": {"25"},
		"mode":     {"1"},
	}
	req, _ := http.NewRequest(http.MethodGet, api+params.Encode(), nil)
	req.Header.Set("Origin", "https://www.iqiyi.com")
	req.Header.Set("Referer", "https://www.iqiyi.com")
	resp, err := c.common.DoReq(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SearchResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	if !result.success() {
		return nil, fmt.Errorf("search error: %d", result.Code)
	}
	if result.Data.Templates == nil || len(result.Data.Templates) <= 0 {
		return nil, fmt.Errorf("search empty templates: %s", keyword)
	}

	var media = make([]*danmaku.Media, 0, len(result.Data.Templates))
	for i, t := range result.Data.Templates {
		if t.Template == 112 {
			// 112 内容聚合页面 单独处理
			for _, intent := range t.IntentAlbumInfos {
				if intent.SiteId != "iqiyi" {
					continue
				}
				year, e := strconv.ParseInt(intent.Superscript, 10, 64)
				if e == nil {
					if !param.MatchYear(int(year)) {
						continue
					}
				}
				match := param.MatchTitle(intent.Title)
				c.common.Logger.Debug(fmt.Sprintf("[%s] match [%s]: %v", intent.Title, param.Title, match))
				if !match {
					continue
				}

				albumMatches := albumRegex.FindStringSubmatch(intent.PlayUrl)
				if len(albumMatches) < 2 {
					continue
				}
				if m, e := c.Media(albumMatches[1]); e == nil {
					media = append(media, m)
				} else {
					c.common.Logger.Error(e.Error())
				}
			}
			continue
		}
		// 过滤非iqiyi平台数据
		if t.AlbumInfo.SiteId != "iqiyi" {
			continue
		}
		if t.Template != 101 && t.Template != 103 {
			continue
		}
		// Subtitle 是年份
		year, err := strconv.ParseInt(t.AlbumInfo.Subtitle, 10, 64)
		if err != nil {
			continue
		}
		if !param.MatchYear(int(year)) {
			continue
		}

		match := param.MatchTitle(t.AlbumInfo.Title)
		c.common.Logger.Debug(fmt.Sprintf("[%s] match [%s]: %v index: %d", t.AlbumInfo.Title, param.Title, match, i))
		if !match {
			continue
		}

		var eps []*danmaku.MediaEpisode
		var mediaType danmaku.MediaType
		var mediaId string
		if t.Template == 101 {
			// 剧集
			if ssId < 0 {
				continue
			}
			if t.AlbumInfo.Videos == nil || len(t.AlbumInfo.Videos) <= 0 {
				continue
			}
			// 匹配 albumId
			albumMatches := albumRegex.FindStringSubmatch(t.AlbumInfo.PlayUrl)
			if len(albumMatches) < 2 {
				continue
			}
			mediaType = danmaku.Series
			eps = make([]*danmaku.MediaEpisode, 0, len(t.AlbumInfo.Videos))
			mediaId = albumMatches[1]
			for _, v := range t.AlbumInfo.Videos {
				epMatches := tvIdRegex.FindStringSubmatch(v.PlayUrl)
				if len(epMatches) < 2 {
					continue
				}
				epTitle := v.Number
				if epTitle == "" {
					epTitle = strconv.FormatInt(int64(i+1), 10)
				}
				eps = append(eps, &danmaku.MediaEpisode{
					Id:        epMatches[1],
					EpisodeId: epTitle,
					Title:     v.Subtitle,
				})
			}

		} else if t.Template == 103 {
			// 电影
			if ssId >= 0 {
				continue
			}
			// 匹配到tvId
			playUrlMatches := tvIdRegex.FindStringSubmatch(t.AlbumInfo.PlayUrl)
			if len(playUrlMatches) < 2 {
				continue
			}
			mediaId = playUrlMatches[1]
			mediaType = danmaku.Movie
			eps = append(eps, &danmaku.MediaEpisode{
				Id:        playUrlMatches[1],
				EpisodeId: t.AlbumInfo.Title,
				Title:     t.AlbumInfo.Title,
			})

		} else {
			//
		}

		m := &danmaku.Media{
			Id:       mediaId,
			Type:     mediaType,
			TypeDesc: t.S3,
			Platform: danmaku.Iqiyi,
			Title:    t.AlbumInfo.Title,
			Desc:     t.AlbumInfo.Introduction,
			Episodes: eps,
		}
		media = append(media, m)
	}

	return media, nil
}

func (c *client) GetDanmaku(id string) ([]*danmaku.StandardDanmaku, error) {
	tvId, err := strconv.ParseInt(id, 10, 64)
	baseInfo, err := c.videoBaseInfo(tvId)
	if err != nil {
		return nil, err
	}
	result := c.scrapeDanmaku(baseInfo, tvId)
	return result, nil
}

func (c *client) Scrape(idStr string) error {
	tvId := parseToNumberId(idStr)
	if tvId <= 0 {
		return nil
	}
	c.common.Logger.Debug(fmt.Sprintf("%s tvid: %d", idStr, tvId))
	baseInfo, err := c.videoBaseInfo(tvId)
	if err != nil {
		return err
	}
	result := c.scrapeDanmaku(baseInfo, tvId)

	serializer := &danmaku.SerializerData{
		EpisodeId:       idStr,
		Data:            result,
		DurationInMills: int64(baseInfo.Data.DurationSec * 1000),
	}

	path := filepath.Join(config.GetConfig().SavePath, danmaku.Iqiyi, strconv.FormatInt(baseInfo.Data.AlbumId, 10))
	title := ""
	if baseInfo.Data.Order > 0 {
		title = strconv.FormatInt(int64(baseInfo.Data.Order), 10) + "_"
	}
	filename := title + strconv.FormatInt(baseInfo.Data.TVId, 10)
	danmaku.WriteFile(danmaku.Iqiyi, serializer, path, filename)

	return nil
}
