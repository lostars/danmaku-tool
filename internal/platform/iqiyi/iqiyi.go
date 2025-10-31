package iqiyi

import (
	"danmu-tool/internal/danmaku"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/andybalholm/brotli"
	"google.golang.org/protobuf/proto"
)

type client struct {
	common *danmaku.PlatformClient
}

func init() {
	danmaku.RegisterInitializer(&client{})
}

func (c *client) Init() error {
	common, err := danmaku.InitPlatformClient(danmaku.Iqiyi)
	if err != nil {
		return err
	}
	c.common = common
	danmaku.RegisterScraper(c)
	return nil
}

func (c *client) Platform() danmaku.Platform {
	return danmaku.Iqiyi
}

/*
	爱奇艺是 使用 albumId 和 tvId，使用转换方法都能转成数字id
	https://www.iqiyi.com/v_19rrk2gwkw.html v_ 后面字符串就是 tvId
	https://www.iqiyi.com/a_19rrk2hct9.html a_ 后面就是 albumId

	不像其他平台，爱奇艺只有剧集才有albumId，电影是没有的。
	电影 albumId 和 tvId 的数字id是相同的
	同时保存弹幕文件使用的是数字id
*/

func (c *client) videoBaseInfo(tvId int64) (*VideoBaseInfoResult, error) {
	// https://cmts.iqiyi.com/bullet/11/00/103411100_60_1_d5a87c30.br
	// https://pcw-api.iqiyi.com/video/video/baseinfo/103411100 视频信息

	baseInfoAPI := "https://pcw-api.iqiyi.com/video/video/baseinfo/" + strconv.FormatInt(tvId, 10)
	req, _ := http.NewRequest(http.MethodGet, baseInfoAPI, nil)
	resp, err := c.common.DoReq(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var baseInfo VideoBaseInfoResult
	e := json.NewDecoder(resp.Body).Decode(&baseInfo)
	if e != nil {
		c.common.Logger.Error(e.Error())
		return nil, e
	}
	if !baseInfo.success() {
		return nil, fmt.Errorf("base info fail code: %s", baseInfo.Code)
	}
	return &baseInfo, nil
}

func (c *client) scrapeDanmaku(baseInfo *VideoBaseInfoResult, tvId int64) []*danmaku.StandardDanmaku {

	duration := baseInfo.Data.DurationSec
	segmentsLen := duration/segmentInterval + 1

	var result []*danmaku.StandardDanmaku

	tasks := make(chan task, segmentsLen)
	lock := sync.Mutex{}
	var wg sync.WaitGroup
	for w := 0; w < c.common.MaxWorker; w++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for t := range tasks {
				data, err := c.scrape(t.tvId, t.segment)
				if err != nil {
					c.common.Logger.Error(fmt.Sprintf("%d scrape segment %d error: %s", tvId, t.segment, err.Error()))
					continue
				}
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
		for i := 1; i <= segmentsLen; i++ {
			tasks <- task{
				tvId:    baseInfo.Data.TVId,
				segment: i,
			}
		}
		close(tasks)
	}()

	wg.Wait()

	return result
}

type task struct {
	tvId    int64
	segment int
}

func (c *client) scrape(tvId int64, segment int) ([]*danmaku.StandardDanmaku, error) {
	req, _ := http.NewRequest(http.MethodGet, buildSegmentUrl(tvId, segment), nil)
	resp, err := c.common.DoReq(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("scrape danmaku error: %s", resp.Status)
	}

	body, err := io.ReadAll(brotli.NewReader(resp.Body))
	if err != nil {
		return nil, err
	}
	danmu := &Danmu{}
	if e := proto.Unmarshal(body, danmu); e != nil {
		return nil, e
	}
	if danmu.Code != "A00000" {
		return nil, fmt.Errorf("tvId: %d scrape fail, segment: %d", tvId, segment)
	}

	var result = make([]*danmaku.StandardDanmaku, 0, len(danmu.Entry))
	for _, b := range danmu.Entry {
		if b.BulletInfo == nil {
			continue
		}
		for _, info := range b.BulletInfo {
			offsetInSeconds, err := strconv.ParseFloat(info.ShowTime, 64)
			if err != nil {
				continue
			}
			colorValue := danmaku.WhiteColor
			value, err := strconv.ParseUint(info.Color, 16, 32)
			if err == nil {
				colorValue = int(value)
			}
			result = append(result, &danmaku.StandardDanmaku{
				Content:     info.Content,
				Color:       colorValue,
				OffsetMills: int64(offsetInSeconds * 1000),
				Mode:        danmaku.RollMode,
			})
		}
	}

	return result, nil
}

type xmlParser struct {
	// 弹幕数据
	danmaku           []*danmaku.StandardDanmaku
	tvId              string
	durationInSeconds int64
}

func (c *xmlParser) Parse() (*danmaku.DataXML, error) {
	if c.danmaku == nil {
		return nil, fmt.Errorf("danmaku is nil")
	}

	xml := danmaku.DataXML{
		ChatServer:     "chat.iqiyi.com",
		ChatID:         c.tvId,
		Mission:        0,
		MaxLimit:       2000,
		Source:         "k-v",
		SourceProvider: danmaku.Iqiyi,
		DataSize:       len(c.danmaku),
		Danmaku:        danmaku.NormalConvert(c.danmaku, danmaku.Iqiyi, c.durationInSeconds*1000),
	}

	return &xml, nil
}
