package iqiyi

import (
	"crypto/md5"
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/andybalholm/brotli"
	"google.golang.org/protobuf/proto"
)

func init() {
	danmaku.RegisterInitializer(&client{})
}

type client struct {
	common *danmaku.PlatformClient
}

func (c *client) Platform() danmaku.Platform {
	return danmaku.Iqiyi
}

func (c *client) Scrape(id interface{}) error {
	if id == nil {
		return danmaku.PlatformError(danmaku.Iqiyi, "invalid params")
	}
	idStr, ok := id.(string)
	if !ok {
		return danmaku.PlatformError(danmaku.Iqiyi, "invalid params")
	}

	baseInfo, err := c.videoBaseInfo(idStr)
	if err != nil {
		return err
	}
	result := c.scrapeDanmaku(baseInfo, idStr)

	parser := &xmlParser{
		tvId:              idStr,
		danmaku:           result,
		durationInSeconds: int64(baseInfo.Data.DurationSec),
	}

	path := filepath.Join(config.GetConfig().SavePath, danmaku.Iqiyi, strconv.FormatInt(baseInfo.Data.AlbumId, 10))
	title := ""
	if baseInfo.Data.Order > 0 {
		title = strconv.FormatInt(int64(baseInfo.Data.Order), 10) + "_"
	}
	filename := title + strconv.FormatInt(baseInfo.Data.TVId, 10)
	if e := c.common.XmlPersist.WriteToFile(parser, path, filename); e != nil {
		return e
	}

	return nil
}

/*
	爱奇艺是 使用 albumId 和 tvId，使用转换方法都能转成数字id
	https://www.iqiyi.com/v_19rrk2gwkw.html v_ 后面字符串就是 tvId
	https://www.iqiyi.com/a_19rrk2hct9.html a_ 后面就是 albumId

	不像其他平台，爱奇艺只有剧集才有albumId，电影是没有的。
	电影 albumId 和 tvId 的数字id是相同的
	同时保存弹幕文件使用的是数字id
*/

func (c *client) videoBaseInfo(tvId string) (*VideoBaseInfoResult, error) {
	// https://cmts.iqiyi.com/bullet/11/00/103411100_60_1_d5a87c30.br
	// https://pcw-api.iqiyi.com/video/video/baseinfo/103411100 视频信息
	id := parseToNumberId(tvId)
	if id <= 0 {
		return nil, fmt.Errorf("wrong format tvId: %s", tvId)
	}

	baseInfoAPI := "https://pcw-api.iqiyi.com/video/video/baseinfo/" + strconv.FormatInt(id, 10)
	req, _ := http.NewRequest(http.MethodGet, baseInfoAPI, nil)
	resp, err := c.common.HttpClient.Do(req)
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

func (c *client) scrapeDanmaku(baseInfo *VideoBaseInfoResult, tvId string) []*danmaku.StandardDanmaku {

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
				data, err := c.scrape(t.tvId, 40)
				if err != nil {
					c.common.Logger.Error(fmt.Sprintf("%s scrape segment %d error: %s", tvId, t.segment, err.Error()))
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

func buildSegmentUrl(tvId int64, segment int) string {
	// https://cmts.iqiyi.com/bullet/11/00/103411100_60_1_d5a87c30.br

	// build path
	path1 := "00"
	path2 := "00"
	tvIdStr := strconv.FormatInt(tvId, 10)
	l := len(tvIdStr)
	if l >= 4 {
		path1 = tvIdStr[l-4 : l-2]
	}
	if l >= 2 {
		path2 = tvIdStr[l-2:]
	}

	// build hash
	input := fmt.Sprintf("%s_%d_%d%s", tvIdStr, segmentInterval, segment, segmentSalt)
	sum := md5.Sum([]byte(input))
	hash := fmt.Sprintf("%x", sum)
	if len(hash) >= 8 {
		hash = hash[len(hash)-8:]
	}
	segmentStr := strconv.FormatInt(int64(segment), 10)
	api := fmt.Sprintf("https://cmts.iqiyi.com/bullet/%s/%s/%s_%d_%s_%s.br", path1, path2, tvIdStr, segmentInterval, segmentStr, hash)

	return api
}

func (c *client) scrape(tvId int64, segment int) ([]*danmaku.StandardDanmaku, error) {
	req, _ := http.NewRequest(http.MethodGet, buildSegmentUrl(tvId, segment), nil)
	resp, err := c.common.HttpClient.Do(req)
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
	var result = make([]*danmaku.StandardDanmaku, 0, len(danmu.Entry))
	for _, b := range danmu.Entry {
		if b.BulletInfo == nil {
			continue
		}
		for _, info := range b.BulletInfo {
			offset, err := strconv.ParseInt(info.ShowTime, 10, 64)
			if err != nil {
				continue
			}
			colorValue := danmaku.WhiteColor
			value, err := strconv.ParseUint(info.Color, 16, 32)
			if err == nil {
				colorValue = int(value)
			}
			result = append(result, &danmaku.StandardDanmaku{
				Content: info.Content,
				Color:   colorValue,
				Offset:  offset,
				Mode:    danmaku.RollMode,
			})
		}
	}

	return result, nil
}

func (c *client) Init() error {
	common, err := danmaku.InitPlatformClient(danmaku.Iqiyi)
	if err != nil {
		return err
	}
	c.common = common
	danmaku.RegisterScraper(c)
	//danmaku.RegisterMediaSearcher(c)
	return nil
}

type xmlParser struct {
	// 弹幕数据
	danmaku           []*danmaku.StandardDanmaku
	tvId              string
	durationInSeconds int64
}

func (c *xmlParser) Parse() (*danmaku.DataXML, error) {
	if c.danmaku == nil {
		return nil, danmaku.PlatformError(danmaku.Iqiyi, "danmaku is nil")
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
