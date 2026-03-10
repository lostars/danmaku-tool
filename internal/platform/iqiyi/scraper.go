package iqiyi

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
)

func (c *client) Match(param danmaku.MatchParam) ([]*danmaku.Media, error) {
	keyword := param.Title
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
	resp, err := c.DoReq(req)
	if err != nil {
		return nil, err
	}

	var result SearchResult
	err = utils.SafeDecodeOkResp(resp, &result)
	if err != nil {
		return nil, err
	}
	if !result.success() {
		return nil, fmt.Errorf("search error: %d", result.Code)
	}
	if len(result.Data.Templates) <= 0 {
		return nil, fmt.Errorf("search empty templates: %s", keyword)
	}

	var media = make([]*danmaku.Media, 0, len(result.Data.Templates))
	for _, t := range result.Data.Templates {
		if t.Template == 112 {
			// 112 内容聚合页面 单独处理
			for _, intent := range t.IntentAlbumInfos {
				// 聚合页面暂时不处理 siteId
				// t.S3 = "专辑列表竖图" 不会返回 siteId

				// 匹配albumId
				albumMatches := albumRegex.FindStringSubmatch(intent.PlayUrl)
				mediaId := ""
				epCount := int64(0)
				if len(albumMatches) >= 2 {
					mediaId = albumMatches[1]
					epNumbers := epNumberRegex.FindStringSubmatch(intent.SubscriptContent)
					if len(epNumbers) > 2 {
						epCount, _ = strconv.ParseInt(epNumbers[1], 10, 64)
					} else {
						utils.ErrorLog(danmaku.Iqiyi, fmt.Sprintf("[%s] not match ep count number", intent.SubscriptContent))
					}
				} else {
					// 默认电影为一集
					epCount = 1
					// 如果匹配不到则是电影一类，匹配tvId
					playUrlMatches := tvIdRegex.FindStringSubmatch(intent.PlayUrl)
					if len(playUrlMatches) < 2 {
						continue
					}
					mediaId = base64.StdEncoding.EncodeToString([]byte(playUrlMatches[1]))
				}
				year, _ := strconv.ParseInt(intent.Superscript, 10, 64)
				matchParam := danmaku.InternalMatchParam{
					Title:   intent.Title,
					Year:    int(year),
					MediaId: mediaId,
				}
				match := param.Match(matchParam)
				utils.DebugLog(danmaku.Iqiyi, fmt.Sprintf("[%s] match [%s]: %v", intent.Title, param.Title, match))
				if !match {
					continue
				}
				if param.Mode == danmaku.Search {
					m := &danmaku.Media{
						Id:           mediaId,
						InternalType: intent.Channel,
						Platform:     danmaku.Iqiyi,
						Title:        intent.Title,
						Cover:        intent.Img,
						Year:         int(year),
						Desc:         intent.PromptDesc,
						EpisodeCount: int(epCount),
					}
					m.MediaType(c)
					media = append(media, m)
				} else {
					if m, e := c.Media(mediaId); e == nil {
						m.Year = int(year)
						m.EpisodeCount = len(m.Episodes)
						media = append(media, m)
					} else {
						utils.ErrorLog(danmaku.Iqiyi, e.Error())
					}
				}
			}
			continue
		}
		// 过滤非iqiyi平台数据
		if t.AlbumInfo.SiteId != "iqiyi" {
			continue
		}
		if !t.validTemplate() {
			continue
		}
		albumMatches := albumRegex.FindStringSubmatch(t.AlbumInfo.PlayUrl)
		mediaId := ""
		if len(albumMatches) >= 2 {
			mediaId = albumMatches[1]
		}
		// Subtitle 是年份
		year, _ := strconv.ParseInt(t.AlbumInfo.Subtitle, 10, 64)
		matchParam := danmaku.InternalMatchParam{
			Title:   t.AlbumInfo.Title,
			Year:    int(year),
			MediaId: mediaId,
		}
		match := param.Match(matchParam)
		utils.DebugLog(danmaku.Iqiyi, fmt.Sprintf("[%s] match [%s]: %v", t.AlbumInfo.Title, param.Title, match))
		if !match {
			continue
		}

		var eps = make([]*danmaku.MediaEpisode, 0, 200)
		if t.AlbumInfo.Videos != nil {
			// 剧集
			if len(t.AlbumInfo.Videos) <= 0 {
				continue
			}
			// 匹配 albumId
			if len(albumMatches) < 2 {
				continue
			}
			eps = make([]*danmaku.MediaEpisode, 0, len(t.AlbumInfo.Videos))
			mediaId = albumMatches[1]
			for _, v := range t.AlbumInfo.Videos {
				epMatches := tvIdRegex.FindStringSubmatch(v.PlayUrl)
				if len(epMatches) < 2 {
					continue
				}
				if danmaku.InvalidEpTitle(v.Subtitle) {
					continue
				}
				// 如果不是数字类型，则可能是花絮一类
				if _, e := strconv.ParseInt(v.Number, 10, 64); e != nil {
					continue
				}
				eps = append(eps, &danmaku.MediaEpisode{
					Id:        epMatches[1],
					EpisodeId: v.Number,
					Title:     v.Subtitle,
				})
			}
		} else {
			// 匹配到tvId
			playUrlMatches := tvIdRegex.FindStringSubmatch(t.AlbumInfo.PlayUrl)
			if len(playUrlMatches) < 2 {
				continue
			}
			// 电影没有albumId 使用的是base64(tvId) 方便在获取详情时区分是albumId还是tvId
			mediaId = base64.StdEncoding.EncodeToString([]byte(playUrlMatches[1]))
			eps = append(eps, &danmaku.MediaEpisode{
				Id:        playUrlMatches[1],
				EpisodeId: t.AlbumInfo.Title,
				Title:     t.AlbumInfo.Title,
			})
		}

		m := &danmaku.Media{
			Id:           mediaId,
			InternalType: t.AlbumInfo.Channel,
			Platform:     danmaku.Iqiyi,
			Title:        t.AlbumInfo.Title,
			Cover:        t.AlbumInfo.Img,
			Year:         int(year),
			Desc:         t.AlbumInfo.Introduction,
			Episodes:     eps,
			EpisodeCount: len(eps),
		}
		m.MediaType(c)
		media = append(media, m)
	}

	return media, nil
}

func (c *client) GetDanmaku(id string) ([]*danmaku.StandardDanmaku, error) {
	tvId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, err
	}
	baseInfo, err := c.videoBaseInfo(tvId)
	if err != nil {
		return nil, err
	}
	result := c.scrapeDanmaku(baseInfo, tvId)
	return result, nil
}

func (c *client) Scrape(idStr string) error {
	// https://www.iqiyi.com/a_15qrt88gex9.html albumId: 15qrt88gex9
	showId := c.ParseNumber(idStr)
	if showId > 0 {
		media, err := c.Media(strconv.FormatInt(showId, 10))
		if err == nil {
			if len(media.Episodes) < 1 {
				return fmt.Errorf("get album failed, invalid album id")
			}
			for _, ep := range media.Episodes {
				if err := c.scrapeById(ep.Id); err != nil {
					utils.ErrorLog(danmaku.Iqiyi, fmt.Sprintf("%s scrape failed: %s", ep.Id, err.Error()))
				}
			}
			return nil
		}
	}

	return c.scrapeById(idStr)
}

func (c *client) scrapeById(idStr string) error {
	// 支持数字tvId
	tvId, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		tvId = c.ParseNumber(idStr)
		if tvId <= 0 {
			return fmt.Errorf("invalid id: %s", idStr)
		}
	}
	utils.DebugLog(danmaku.Iqiyi, fmt.Sprintf("%s tvid: %d", idStr, tvId))
	baseInfo, err := c.videoBaseInfo(tvId)
	if err != nil {
		return err
	}

	path := filepath.Join(config.GetConfig().SavePath, danmaku.Iqiyi, strconv.FormatInt(baseInfo.Data.AlbumId, 10))
	serializer := &danmaku.SerializerData{
		EpisodeId:       strconv.FormatInt(baseInfo.Data.TVId, 10),
		SeasonId:        strconv.FormatInt(baseInfo.Data.AlbumId, 10),
		DurationInMills: int64(baseInfo.Data.DurationSec * 1000),
		Platform:        danmaku.Iqiyi,
		FullPath:        path,
		Filename:        strconv.FormatInt(baseInfo.Data.TVId, 10),
	}
	if err = danmaku.CheckFile(serializer); err != nil {
		return err
	}

	serializer.Data = c.scrapeDanmaku(baseInfo, tvId)

	danmaku.WriteFile(serializer)

	return nil
}
