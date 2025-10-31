package tencent

import (
	"bytes"
	"danmu-tool/internal/danmaku"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
)

func (c *client) Init() error {
	common, err := danmaku.InitPlatformClient(danmaku.Tencent)
	if err != nil {
		return err
	}
	c.common = common
	danmaku.RegisterScraper(c)
	return nil
}

func init() {
	danmaku.RegisterInitializer(&client{})
}

type client struct {
	common *danmaku.PlatformClient
}

func (c *client) Platform() danmaku.Platform {
	return danmaku.Tencent
}

func (c *client) doSeriesRequest(cid, vid string, pageId, pageContent string) (*SeriesResult, error) {
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
	seriesResp, err := c.common.DoReq(seriesReq)
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

func (c *client) series(cid string) ([]*SeriesItem, error) {
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
		c.common.Logger.Debug("series has no tabs", "cid", cid)
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
			c.common.Logger.Error(err.Error())
			continue
		}

		d, err := tabSeries.series()
		if err != nil {
			c.common.Logger.Error(err.Error())
			continue
		}
		eps = append(eps, d...)
	}

	return eps, nil
}

func (c *client) getDanmakuByVid(vid string) ([]*danmaku.StandardDanmaku, error) {
	param := map[string]string{
		"vid":            vid,
		"engine_version": "2.1.10",
	}
	configBytes, err := json.Marshal(param)
	if err != nil {
		return nil, err
	}
	configAPI := "https://pbaccess.video.qq.com/trpc.barrage.custom_barrage.CustomBarrage/GetDMStartUpConfig"
	danmakuConfigReq, err := http.NewRequest(http.MethodPost, configAPI, bytes.NewBuffer(configBytes))
	if err != nil {
		return nil, err
	}
	c.setRequest(danmakuConfigReq)
	resp, e := c.common.DoReq(danmakuConfigReq)
	if e != nil {
		return nil, e
	}
	var segmentResult DanmakuSegmentResult
	e = json.NewDecoder(resp.Body).Decode(&segmentResult)
	if e != nil {
		return nil, e
	}
	resp.Body.Close()
	var segmentsLen = len(segmentResult.Data.SegmentIndex)
	if segmentResult.Data.SegmentIndex == nil || segmentsLen <= 0 {
		return nil, fmt.Errorf("no segments vid: %s", vid)
	}
	c.common.Logger.Debug(fmt.Sprintf("danmaku segments size: %v", segmentsLen), "vid", vid, "size", segmentsLen)

	var result []*danmaku.StandardDanmaku
	lock := sync.Mutex{}
	tasks := make(chan task, segmentsLen)
	var wg sync.WaitGroup
	for w := 0; w < c.common.MaxWorker; w++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for t := range tasks {
				data := c.scrape(t.vid, t.segment)
				if data == nil || len(data) <= 0 {
					continue
				}
				lock.Lock()
				result = append(result, data...)
				lock.Unlock()
			}
		}(w)
	}

	go func() {
		for _, v := range segmentResult.Data.SegmentIndex {
			tasks <- task{
				vid:     vid,
				segment: v.SegmentName,
			}
		}
		close(tasks)
	}()

	wg.Wait()

	return result, nil
}

type task struct {
	vid     string
	segment string
}

func (c *client) scrape(vid, segment string) []*danmaku.StandardDanmaku {
	//https://dm.video.qq.com/barrage/segment/{vid}/{segment}
	api := fmt.Sprintf("https://dm.video.qq.com/barrage/segment/%s/%s", vid, segment)

	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		c.common.Logger.Error(err.Error())
		return nil
	}
	resp, err := c.common.DoReq(req)
	if err != nil {
		c.common.Logger.Error(err.Error())
		return nil
	}
	defer resp.Body.Close()

	var danmakuResult DanmakuResult
	err = json.NewDecoder(resp.Body).Decode(&danmakuResult)
	if err != nil {
		c.common.Logger.Error(err.Error())
		return nil
	}

	var result = make([]*danmaku.StandardDanmaku, 0, len(danmakuResult.BarrageList))
	for _, v := range danmakuResult.BarrageList {
		offset, err := strconv.ParseInt(v.TimeOffset, 10, 64)
		if err != nil {
			c.common.Logger.Error("invalid offset", "vid", vid, "offset", v.TimeOffset)
			continue
		}

		// 解析颜色
		var color DanmakuColorResult
		var mode = danmaku.RollMode
		var colorValue = danmaku.WhiteColor
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
			Content:     v.Content,
			OffsetMills: offset,
			Mode:        mode,
			Color:       colorValue,
			Platform:    danmaku.Tencent,
		}
		result = append(result, r)
	}

	return result
}

func (c *client) setRequest(req *http.Request) {
	req.Header.Set("Cookie", c.common.Cookie)
	req.Header.Set("Origin", "https://v.qq.com/")
	req.Header.Set("Referer", "https://v.qq.com/")
	// 注意如果json请求不设置该请求头，则会导致部分接口异常返回400，哪怕参数全部正常。
	// 可能同一个接口，参数换一个值就不行了。
	if req.Method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
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
