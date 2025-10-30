package iqiyi

import (
	"danmu-tool/internal/danmaku"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
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
	for _, t := range result.Data.Templates {
		// 过滤非iqiyi平台数据
		if t.AlbumInfo.SiteId != "iqiyi" {
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

		match := fuzzy.Match(t.AlbumInfo.Title, keyword)
		c.common.Logger.Debug(fmt.Sprintf("%s match %s: %v", t.AlbumInfo.Title, keyword, match))
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
				eps = append(eps, &danmaku.MediaEpisode{
					Id:        epMatches[1],
					EpisodeId: v.Number,
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
			continue
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

var tvIdRegex = regexp.MustCompile(`^qips://tvid=(\d+);`)
var albumRegex = regexp.MustCompile(`albumid=(\d+);$`)

func (c *client) GetDanmaku(id string) ([]*danmaku.StandardDanmaku, error) {
	// [platform]_[id]_[id]
	s := strings.Split(id, "_")
	if len(s) != 3 {
		return nil, danmaku.PlatformError(danmaku.Iqiyi, "invalid id")
	}
	tvId, err := strconv.ParseInt(s[2], 10, 64)
	baseInfo, err := c.videoBaseInfo(tvId)
	if err != nil {
		return nil, err
	}
	result := c.scrapeDanmaku(baseInfo, tvId)
	return result, nil
}

func (c *client) SearcherType() danmaku.Platform {
	return danmaku.Iqiyi
}
