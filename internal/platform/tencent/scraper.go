package tencent

import (
	"bytes"
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
)

func (c *client) Match(param danmaku.MatchParam) ([]*danmaku.Media, error) {
	keyword := param.FileName
	ssId := int64(param.SeasonId)

	c.common.Logger.Debug(fmt.Sprintf("search keyword: %s", keyword))
	searchParam := SearchParam{
		Version:    "25101301",
		ClientType: 1,
		Query:      keyword,
		PageNum:    0,
		IsPrefetch: true,
		PageSize:   30,
		QueryFrom:  102,
		NeedQc:     true,
		ExtraInfo: SearchExtraInfo{
			IsNewMarkLabel:  "1",
			MultiTerminalPc: "1",
			ThemeType:       "1",
		},
	}

	paramBytes, err := json.Marshal(searchParam)
	if err != nil {
		return nil, err
	}
	searchAPI := "https://pbaccess.video.qq.com/trpc.videosearch.mobile_search.MultiTerminalSearch/MbSearch?vversion_platform=2"
	searchReq, err := http.NewRequest(http.MethodPost, searchAPI, bytes.NewBuffer(paramBytes))
	if err != nil {
		return nil, err
	}
	c.setRequest(searchReq)

	resp, err := c.common.DoReq(searchReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResult SearchResult
	err = json.NewDecoder(resp.Body).Decode(&searchResult)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("search http status: %s", resp.Status))
	}
	if searchResult.Ret != 0 {
		return nil, errors.New(fmt.Sprintf("search ret code: %v %s", searchResult.Ret, searchResult.Msg))
	}

	var itemList []SearchResultItem
	if searchResult.Data.NormalList.ItemList != nil {
		itemList = append(itemList, searchResult.Data.NormalList.ItemList...)
	}
	if searchResult.Data.AreaBoxList != nil {
		for _, v := range searchResult.Data.AreaBoxList {
			// 有时候从normalList不出数据，需要从areaBoxList中获取
			if v.BoxId == "MainNeed" && v.ItemList != nil {
				itemList = append(itemList, v.ItemList...)
			}
		}
	}

	var result []*danmaku.Media
	for _, v := range itemList {
		if tencentExcludeRegex.MatchString(v.VideoInfo.SubTitle) {
			// 命中黑名单 则代表搜索不到
			c.common.Logger.Info("title in blacklist", "subTitle", v.VideoInfo.SubTitle)
			continue
		}
		if !param.MatchYear(v.VideoInfo.Year) {
			continue
		}

		clearTitle := danmaku.ClearTitle(v.VideoInfo.Title)
		match := danmaku.Tokenizer.Match(clearTitle, keyword)
		c.common.Logger.Debug(fmt.Sprintf("[%s] match [%s]: %v", clearTitle, keyword, match))
		if !match {
			continue
		}

		var mediaType danmaku.MediaType
		if ssId < 0 {
			mediaType = danmaku.Movie
		} else {
			if v.VideoInfo.SubjectDoc.VideoNum <= 0 {
				// 没有集数信息
				continue
			}
			mediaType = danmaku.Series
		}

		seriesItems, e := c.series(v.Doc.Id)
		if e != nil {
			c.common.Logger.Error(e.Error())
			continue
		}

		var eps = make([]*danmaku.MediaEpisode, 0, v.VideoInfo.SubjectDoc.VideoNum)
		for _, ep := range seriesItems {
			if ep.ItemParams.IsTrailer == "1" {
				continue
			}
			// 有可能vid为空
			if ep.ItemParams.VID == "" {
				continue
			}
			eps = append(eps, &danmaku.MediaEpisode{
				Id:        ep.ItemParams.VID,
				EpisodeId: ep.ItemParams.Title,
				Title:     ep.ItemParams.CTitleOutput,
			})
		}
		// 匹配剧场版 epId 暂时使用下标作为S00的epId 最新发布的在最前面
		if ssId == 0 {
			for i, ep := range eps {
				ep.EpisodeId = strconv.FormatInt(int64(len(eps)-i), 10)
			}
		}

		media := &danmaku.Media{
			Id:       v.Doc.Id,
			Type:     mediaType,
			TypeDesc: v.VideoInfo.TypeName,
			Desc:     v.VideoInfo.Desc,
			Title:    v.VideoInfo.Title,
			Episodes: eps,
			Platform: danmaku.Tencent,
		}

		result = append(result, media)
	}

	return result, nil
}

func (c *client) GetDanmaku(id string) ([]*danmaku.StandardDanmaku, error) {
	return c.getDanmakuByVid(id)
}

func (c *client) Scrape(idStr string) error {
	var isVID = len(idStr) == 11
	var cid = idStr
	// 是否只查询单集 如果是vid且获取到对应的cid，则只查询该ep
	var onlyCurrentVID = false
	if isVID {
		// 反查cid 然后再继续查询剧集
		series, err := c.doSeriesRequest("", idStr, SeriesInfoPageId, "")
		if err != nil {
			return err
		}
		infos, err := series.series()
		if err != nil {
			return err
		}
		epCID := infos[0].ItemParams.ReportCID
		if epCID == "" {
			return fmt.Errorf("%s has no cid", idStr)
		}
		cid = epCID
		onlyCurrentVID = true
	}

	eps, err := c.series(cid)
	if err != nil {
		return err
	}
	c.common.Logger.Info("get ep done", "cid", cid, "size", len(eps))
	if len(eps) <= 0 {
		return nil
	}

	for _, ep := range eps {
		// 只获取对应ep数据
		if onlyCurrentVID && ep.ItemParams.VID != idStr {
			continue
		}
		if ep.ItemParams.IsTrailer == "1" {
			c.common.Logger.Info("ep skipped because of trailer type", "vid", ep.ItemParams.VID)
			continue
		}
		// 有可能vid为空
		if ep.ItemParams.VID == "" {
			c.common.Logger.Debug("skipped because of empty vid")
			continue
		}

		data, e := c.getDanmakuByVid(ep.ItemParams.VID)
		if e != nil {
			c.common.Logger.Error(fmt.Sprintf("get danmaku by vid error: %s", e.Error()))
			continue
		}
		parser := &xmlParser{
			vid:     ep.ItemParams.VID,
			danmaku: data,
		}
		v, err := strconv.ParseInt(ep.ItemParams.Duration, 10, 64)
		if err == nil {
			parser.durationInMills = v * 1000
		} else {
			c.common.Logger.Error("duration is not number", "vid", ep.ItemParams.VID, "duration", ep.ItemParams.Duration)
		}

		path := filepath.Join(config.GetConfig().SavePath, danmaku.Tencent, ep.ItemParams.CID)
		title := ""
		if _, err := strconv.ParseInt(ep.ItemParams.Title, 10, 64); err == nil {
			title = ep.ItemParams.Title + "_"
		}
		filename := title + ep.ItemParams.VID
		if e := c.common.XmlPersist.WriteToFile(parser, path, filename); e != nil {
			c.common.Logger.Error(e.Error())
		}

		c.common.Logger.Info("ep scraped done", "vid", ep.ItemParams.VID, "size", len(parser.danmaku))
	}

	c.common.Logger.Info("danmaku scraped done", "cid", cid)

	return nil
}
