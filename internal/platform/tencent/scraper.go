package tencent

import (
	"bytes"
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
)

func (c *client) Match(param danmaku.MatchParam) ([]*danmaku.Media, error) {
	keyword := param.Title
	ssId := int64(param.SeasonId)

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

	resp, err := c.DoReq(searchReq)
	if err != nil {
		return nil, err
	}
	defer utils.SafeClose(resp.Body)

	var searchResult SearchResult
	err = json.NewDecoder(resp.Body).Decode(&searchResult)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search http status: %s", resp.Status)
	}
	if searchResult.Ret != 0 {
		return nil, fmt.Errorf("search ret code: %v %s", searchResult.Ret, searchResult.Msg)
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

	// 数据去重
	var data = make([]SearchResultItem, 0, len(itemList))
	var count = make(map[string]bool, len(itemList))
	for _, v := range itemList {
		if count[v.VideoInfo.Title] {
			continue
		}
		data = append(data, v)
		count[v.VideoInfo.Title] = true
	}

	var result []*danmaku.Media
	// 并发处理 循环中需要获取剧集列表 4并发已足够再高就会触发限流
	sem := make(chan struct{}, 4)
	lock := sync.Mutex{}
	wg := sync.WaitGroup{}
	for _, v := range data {
		wg.Add(1)
		sem <- struct{}{}
		go func(v SearchResultItem) {
			defer wg.Done()
			defer func() { <-sem }()

			// 黑名单 基本都是外站视频
			if tencentExcludeRegex.MatchString(v.VideoInfo.SubTitle) {
				return
			}
			if v.VideoInfo.Year <= 0 || !param.MatchYear(v.VideoInfo.Year) {
				return
			}

			match := param.MatchTitle(v.VideoInfo.Title)
			utils.DebugLog(danmaku.Tencent, fmt.Sprintf("[%s] match [%s]: %v", v.VideoInfo.Title, param.Title, match))
			if !match {
				return
			}

			var mediaType danmaku.MediaType
			if v.VideoInfo.TypeName == "电影" {
				mediaType = danmaku.Movie
			} else {
				if v.VideoInfo.SubjectDoc.VideoNum <= 0 {
					// 没有集数信息
					return
				}
				mediaType = danmaku.Series
			}

			seriesItems, e := c.series(v.Doc.Id)
			if e != nil {
				utils.ErrorLog(danmaku.Tencent, e.Error())
				return
			}

			var eps = make([]*danmaku.MediaEpisode, 0, v.VideoInfo.SubjectDoc.VideoNum)
			for i, ep := range seriesItems {
				if ep.ItemParams.IsTrailer == "1" {
					continue
				}
				// 有可能vid为空
				if ep.ItemParams.VID == "" {
					continue
				}
				epTitle := ep.ItemParams.CTitleOutput
				if epId, e := strconv.ParseInt(epTitle, 10, 64); e == nil {
					epTitle = strconv.FormatInt(epId, 10)
				}
				if epTitle == "" {
					epTitle = strconv.FormatInt(int64(i+1), 10)
				}
				eps = append(eps, &danmaku.MediaEpisode{
					Id:        ep.ItemParams.VID,
					EpisodeId: epTitle,
					Title:     ep.ItemParams.Title,
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
				Cover:    v.VideoInfo.ImgUrl,
				Year:     v.VideoInfo.Year,
				Episodes: eps,
				Platform: danmaku.Tencent,
			}

			lock.Lock()
			result = append(result, media)
			lock.Unlock()
		}(v)

	}
	wg.Wait()

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
	utils.InfoLog(danmaku.Tencent, "get ep done", "cid", cid, "size", len(eps))
	if len(eps) <= 0 {
		return nil
	}

	for _, ep := range eps {
		// 只获取对应ep数据
		if onlyCurrentVID && ep.ItemParams.VID != idStr {
			continue
		}
		if !ep.validEP() {
			continue
		}

		data, e := c.getDanmakuByVid(ep.ItemParams.VID)
		if e != nil {
			utils.ErrorLog(danmaku.Tencent, fmt.Sprintf("get danmaku by vid error: %s", e.Error()))
			continue
		}
		serializer := &danmaku.SerializerData{
			EpisodeId: ep.ItemParams.VID,
			Data:      data,
		}
		v, err := strconv.ParseInt(ep.ItemParams.Duration, 10, 64)
		if err == nil {
			serializer.DurationInMills = v * 1000
		} else {
			utils.ErrorLog(danmaku.Tencent, "duration is not number", "vid", ep.ItemParams.VID, "duration", ep.ItemParams.Duration)
		}

		path := filepath.Join(config.GetConfig().SavePath, danmaku.Tencent, ep.ItemParams.CID)
		danmaku.WriteFile(danmaku.Tencent, serializer, path, ep.ItemParams.VID)

		utils.InfoLog(danmaku.Tencent, "ep scraped done", "vid", ep.ItemParams.VID, "size", len(data))
	}

	utils.InfoLog(danmaku.Tencent, "danmaku scraped done", "cid", cid)

	return nil
}
