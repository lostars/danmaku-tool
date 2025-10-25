package tencent

import (
	"bytes"
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Client struct {
	HttpClient *http.Client
	MaxWorker  int
	Cookie     string

	DataPersists []danmaku.DataPersist
	// 弹幕数据
	danmaku     []*danmaku.StandardDanmaku
	vid         string
	danmakuLock sync.Mutex
	duration    int64
}

func (c *Client) Parse() (*danmaku.DataXML, error) {
	if c.danmaku == nil {
		return nil, danmaku.PlatformError(danmaku.Tencent, "danmaku is nil")
	}

	var source = c.danmaku
	if config.GetConfig().Tencent.MergeDanmakuInMills > 0 {
		if c.duration > 0 {
			var merged = danmaku.MergeDanmaku(source, config.GetConfig().Tencent.MergeDanmakuInMills, c.duration)
			source = make([]*danmaku.StandardDanmaku, 0, len(merged))
			for _, v := range merged {
				source = append(source, v)
			}
		} else {
			logger.Error("vid: %s duration is 0\n", c.vid)
		}
	}

	var data = make([]danmaku.DataXMLDanmaku, len(source))
	// <d p="2.603,1,25,16777215,[tencent]">看看 X2</d>
	// 第几秒/弹幕类型/字体大小/颜色
	for i, v := range source {
		var attr = []string{
			strconv.FormatFloat(float64(v.Offset)/1000, 'f', 2, 64),
			strconv.FormatInt(int64(v.Mode), 10),
			"25", // 腾讯视频弹幕没有字体大小，固定25
			strconv.FormatInt(int64(v.Color), 10),
			fmt.Sprintf("[%s]", c.Platform()),
		}
		d := danmaku.DataXMLDanmaku{
			Attributes: strings.Join(attr, ","),
			Content:    v.Content,
		}
		data[i] = d
	}

	xml := danmaku.DataXML{
		ChatServer:     "comment.bilibili.com",
		ChatID:         c.vid,
		Mission:        0,
		MaxLimit:       2000,
		Source:         "k-v",
		SourceProvider: danmaku.Tencent,
		DataSize:       len(source),
		Danmaku:        data,
	}

	return &xml, nil
}

func (c *Client) Platform() danmaku.Platform {
	return danmaku.Tencent
}

func (c *Client) doSeriesRequest(cid, vid string, pageId, pageContent string) (*SeriesResult, error) {
	var seriesReqParam = SeriesReqParam{
		HasCache: 1,
		PageParams: SeriesReqPageParam{
			ReqFrom:        "web_vsite",
			PageId:         pageId,
			PageType:       "detail_operation",
			IdType:         "1",
			CID:            cid,
			VID:            vid,
			DetailPageType: "1",
			PageContext:    pageContent,
		},
	}
	jsonBytes, err := json.Marshal(seriesReqParam)
	if err != nil {
		return nil, err
	}
	// url 参数需要保留
	seriesAPI := "https://pbaccess.video.qq.com/trpc.universal_backend_service.page_server_rpc.PageServer/GetPageData?video_appid=3000010&vversion_name=8.2.96&vversion_platform=2"
	seriesReq, err := http.NewRequest(http.MethodPost, seriesAPI, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	c.setRequest(seriesReq)
	seriesResp, err := c.HttpClient.Do(seriesReq)
	if err != nil {
		return nil, err
	}
	defer seriesResp.Body.Close()

	var seriesResult SeriesResult
	err = json.NewDecoder(seriesResp.Body).Decode(&seriesResult)
	if err != nil {
		return nil, err
	}
	return &seriesResult, nil
}

func (c *Client) series(cid string) ([]*SeriesItem, error) {
	// 获取剧集信息
	seriesResult, err := c.doSeriesRequest(cid, "", SeriesEPPageId, "")
	if err != nil {
		return nil, err
	}

	// 获取当前tab剧集信息
	var eps = make([]*SeriesItem, 0, 500)
	data, err := seriesResult.series()
	if err != nil {
		return nil, err
	}
	eps = append(eps, data...)

	// 解析剧集信息，可能会有多个tab
	tabStr := seriesResult.Data.ModuleListData[0].ModuleData[0].ModuleParams.Tabs
	if tabStr == "" {
		logger.Error("series has no tabs", "cid", cid)
		return eps, nil
	}
	var tabs []SeriesTab
	err = json.Unmarshal([]byte(tabStr), &tabs)
	if err != nil {
		return nil, err
	}

	for _, tab := range tabs {
		if tab.Selected {
			continue
		}

		tabSeries, err := c.doSeriesRequest(cid, "", SeriesEPPageId, tab.PageContext)
		if err != nil {
			logger.Error(err.Error())
			continue
		}

		d, err := tabSeries.series()
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		eps = append(eps, d...)
	}

	return eps, nil
}

func (c *Client) Scrape(id interface{}) error {
	if id == nil {
		return danmaku.PlatformError(danmaku.Tencent, "nil params")
	}
	idStr, ok := id.(string)
	if !ok {
		return danmaku.PlatformError(danmaku.Tencent, "invalid params")
	}
	var isVID = len(idStr) == 11
	var cid = idStr
	// 是否只查询单集 如果是vid且获取到对应的cid，则只查询该ep
	var onlyCurrentVID = false
	if isVID {
		// 反查cid 然后再继续查询剧集
		series, err := c.doSeriesRequest("", idStr, SeriesInfoPageId, "")
		if err != nil {
			return danmaku.PlatformError(danmaku.Tencent, err.Error())
		}
		infos, err := series.series()
		if err != nil {
			return danmaku.PlatformError(danmaku.Tencent, err.Error())
		}
		epCID := infos[0].ItemParams.ReportCID
		if epCID == "" {
			return danmaku.PlatformError(danmaku.Tencent, fmt.Sprintf("%s has no cid", idStr))
		}
		cid = epCID
		onlyCurrentVID = true
	}

	eps, err := c.series(cid)
	if err != nil {
		return err
	}
	logger.Info("ep size", "cid", cid, "size", len(eps))
	if len(eps) <= 0 {
		return nil
	}

	for _, ep := range eps {
		// 只获取对应ep数据
		if onlyCurrentVID && ep.ItemParams.VID != idStr {
			continue
		}
		if ep.ItemParams.IsTrailer == "1" {
			logger.Info("ep skipped because of trailer type", "vid", ep.ItemParams.VID)
			continue
		}
		// 有可能vid为空
		if ep.ItemParams.VID == "" {
			logger.Debug("skipped because of empty vid")
			continue
		}

		// 获取弹幕分片信息
		param := map[string]string{
			"vid":            ep.ItemParams.VID,
			"engine_version": "2.1.10",
		}
		configBytes, err := json.Marshal(param)
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		configAPI := "https://pbaccess.video.qq.com/trpc.barrage.custom_barrage.CustomBarrage/GetDMStartUpConfig"
		danmakuConfigReq, err := http.NewRequest(http.MethodPost, configAPI, bytes.NewBuffer(configBytes))
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		c.setRequest(danmakuConfigReq)
		resp, e := c.HttpClient.Do(danmakuConfigReq)
		if e != nil {
			logger.Error(e.Error())
			continue
		}
		var segmentResult DanmakuSegmentResult
		e = json.NewDecoder(resp.Body).Decode(&segmentResult)
		if e != nil {
			logger.Error(e.Error())
			continue
		}
		resp.Body.Close()
		var segmentsLen = len(segmentResult.Data.SegmentIndex)
		if segmentResult.Data.SegmentIndex == nil || segmentsLen <= 0 {
			logger.Error("no segments", "vid", ep.ItemParams.VID)
			continue
		}

		c.vid = ep.ItemParams.VID
		logger.Info("danmaku segments", "vid", c.vid, "size", segmentsLen)
		v, err := strconv.ParseInt(ep.ItemParams.Duration, 10, 64)
		if err == nil {
			c.duration = v * 1000
		} else {
			logger.Error("duration is not number\n", "vid", ep.ItemParams.VID, "duration", ep.ItemParams.Duration)
		}
		tasks := make(chan task, segmentsLen)
		var wg sync.WaitGroup
		for w := 0; w < c.MaxWorker; w++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				for t := range tasks {
					data := c.scrape(t.vid, t.segment)
					if data == nil || len(data) <= 0 {
						continue
					}
					c.danmakuLock.Lock()
					c.danmaku = append(c.danmaku, data...)
					c.danmakuLock.Unlock()
				}
			}(w)
		}

		go func() {
			for _, v := range segmentResult.Data.SegmentIndex {
				tasks <- task{
					vid:     ep.ItemParams.VID,
					segment: v.SegmentName,
				}
			}
			close(tasks)
		}()

		wg.Wait()

		path := filepath.Join(config.GetConfig().SavePath, danmaku.Tencent, ep.ItemParams.CID)
		filename := ep.ItemParams.Title + "_" + ep.ItemParams.VID
		for i, persist := range c.DataPersists {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				if e := persist.WriteToFile(path, filename); e != nil {
					logger.Error(e.Error())
				}
			}(i)
		}
		wg.Wait()

		logger.Info("danmaku scraped done", "vid", ep.ItemParams.VID, "size", len(c.danmaku))
	}

	logger.Info("danmaku scraped done", "cid", cid)

	return nil
}

type task struct {
	vid     string
	segment string
}

type DanmakuSegmentResult struct {
	Ret  int    `json:"ret"`
	Msg  string `json:"msg"`
	Data struct {
		SegmentSpan  string `json:"segment_span"`
		SegmentStart string `json:"segment_start"`
		SegmentIndex map[string]struct {
			SegmentStart string `json:"segment_start"`
			SegmentName  string `json:"segment_name"`
		} `json:"segment_index"`
	} `json:"data"`
}

type DanmakuResult struct {
	BarrageList []struct {
		Content    string `json:"content"`
		Id         string `json:"id"`
		UpCount    string `json:"up_count"`    // 点赞数？
		CreateTime string `json:"create_time"` // 1715077975
		// {\"color\":\"ffffff\",\"gradient_colors\":[\"44EB1F\",\"44EB1F\"],\"position\":1}
		// 颜色信息 json
		ContentStyle string `json:"content_style"`
		TimeOffset   string `json:"time_offset"` // 弹幕偏移时间 ms
	} `json:"barrage_list"`
}

type DanmakuColorResult struct {
	Color          string   `json:"color"`
	GradientColors []string `json:"gradient_colors"`
	Position       int      `json:"position"`
}

func (c *Client) scrape(vid, segment string) []*danmaku.StandardDanmaku {
	//https://dm.video.qq.com/barrage/segment/{vid}/{segment}
	api := fmt.Sprintf("https://dm.video.qq.com/barrage/segment/%s/%s", vid, segment)

	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		logger.Error(err.Error())
		return nil
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		logger.Error(err.Error())
		return nil
	}
	defer resp.Body.Close()

	var danmakuResult DanmakuResult
	err = json.NewDecoder(resp.Body).Decode(&danmakuResult)
	if err != nil {
		logger.Error(err.Error())
		return nil
	}

	var result = make([]*danmaku.StandardDanmaku, 0, len(danmakuResult.BarrageList))
	for _, v := range danmakuResult.BarrageList {
		offset, err := strconv.ParseInt(v.TimeOffset, 10, 64)
		if err != nil {
			logger.Error("invalid offset", "vid", vid, "offset", v.TimeOffset)
			continue
		}

		// 解析颜色
		var color DanmakuColorResult
		var mode = danmaku.RollMode
		var colorValue = 16777215
		if err := json.Unmarshal([]byte(v.ContentStyle), &color); err == nil {
			switch color.Position {
			case 2:
				mode = danmaku.TopMode
			case 3:
				mode = danmaku.BottomMode
			}
			var colorStr = color.Color
			if color.GradientColors != nil && len(color.GradientColors) > 0 {
				colorStr = color.GradientColors[0]
			}
			value, err := strconv.ParseUint(colorStr, 16, 32)
			if err == nil {
				colorValue = int(value)
			}
		}

		r := &danmaku.StandardDanmaku{
			Content:  v.Content,
			Offset:   offset,
			Mode:     mode,
			Color:    colorValue,
			Platform: danmaku.Tencent,
		}
		result = append(result, r)
	}

	return result
}

func (c *Client) setRequest(req *http.Request) {
	// 目前腾讯接口不用Cookie
	//req.Header.Set("Cookie", c.Cookie)
	req.Header.Set("Origin", "https://v.qq.com/")
	req.Header.Set("Referer", "https://v.qq.com/")
	// 注意如果json请求不设置该请求头，则会导致部分接口异常返回400，哪怕参数全部正常。
	// 可能同一个接口，参数换一个值就不行了。
	if req.Method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
}

type SeriesResult struct {
	Ret  int    `json:"ret"`
	Msg  string `json:"msg"`
	Data struct {
		ModuleListData []struct {
			ModuleData []struct {
				ModuleParams struct {
					Tabs string `json:"tabs"` // 这是个json字符串，页面上剧集列表里的tab信息，1-30集 31-50集
				} `json:"module_params"`
				ItemDataLists struct {
					ItemData []SeriesItem `json:"item_datas"`
				} `json:"item_data_lists"`
			} `json:"module_datas"`
		} `json:"module_list_datas"`
	} `json:"data"`
}

func (s *SeriesResult) series() ([]*SeriesItem, error) {
	var eps = make([]*SeriesItem, 0, 500)
	// 返回结果检查
	if s.Ret != 0 {
		return nil, errors.New(fmt.Sprintf("series result: %v %s", s.Ret, s.Msg))
	}
	if s.Data.ModuleListData == nil || len(s.Data.ModuleListData) <= 0 {
		return nil, errors.New("empty ModuleListData")
	}
	d := s.Data.ModuleListData[0]
	if d.ModuleData == nil || len(d.ModuleData) <= 0 {
		return nil, errors.New("empty ModuleData")
	}

	piece := d.ModuleData[0].ItemDataLists.ItemData
	if piece == nil || len(piece) <= 0 {
		return eps, nil
	}

	for _, v := range piece {
		eps = append(eps, &v)
	}
	return eps, nil
}

type SeriesTab struct {
	Begin       int    `json:"begin"`
	End         int    `json:"end"`
	Selected    bool   `json:"selected"`
	PageContext string `json:"page_context"` // 用于剧集tab 获取ep的重要参数
	PageNum     string `json:"page_num"`
	PageSize    string `json:"page_size"`
}

const SeriesEPPageId = "vsite_episode_list"
const SeriesInfoPageId = "detail_page_introduction"

type SeriesItem struct {
	ItemId     string `json:"item_id"` // 等于vid？
	ItemType   string `json:"item_type"`
	ItemParams struct {
		// 以下是 page_id=vsite_episode_list 返回的剧集ep信息
		VID          string `json:"vid"`
		Duration     string `json:"duration"`       // 时长：秒
		CTitleOutput string `json:"c_title_output"` // 01
		// 该字段在 page_id=detail_page_introduction 返回剧集名称
		// page_id=vsite_episode_list 返回ep集数
		Title     string `json:"title"`      // 1
		IsTrailer string `json:"is_trailer"` // 1=预告 0=否
		CID       string `json:"cid"`

		// 以下是 page_id=detail_page_introduction 返回的剧集信息
		ReportCID string `json:"report.cid"`
		// 2=剧集 1=电影 10=综艺 3=动漫 9=纪录片 4=体育
		Type string `json:"type"`
		// 剧集集数
		EpisodeAll string `json:"episode_all"`
		// 1=已完结？需要确认
		AnimeUpdateStatusId string `json:"anime_update_status_id"`
	} `json:"item_params"`
}

type SeriesReqParam struct {
	HasCache   int                `json:"has_cache"`
	PageParams SeriesReqPageParam `json:"page_params"`
}
type SeriesReqPageParam struct {
	ReqFrom string `json:"req_from"`
	// detail_page_introduction 获取剧集信息
	// vsite_episode_list 剧集ep信息
	PageId         string `json:"page_id"`
	PageType       string `json:"page_type"`
	IdType         string `json:"id_type"`
	PageSize       string `json:"page_size"`
	CID            string `json:"cid"` // 剧集id
	VID            string `json:"vid"` // 视频id
	LID            string `json:"lid"`
	PageNum        string `json:"page_num"`
	PageContext    string `json:"page_context"` // 这是个json字符串，页面上剧集列表里的tab信息，1-30集 31-50集
	DetailPageType string `json:"detail_page_type"`
}

var logger *slog.Logger
