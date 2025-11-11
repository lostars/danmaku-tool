package iqiyi

import (
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

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
	defer utils.SafeClose(resp.Body)

	var baseInfo VideoBaseInfoResult
	e := json.NewDecoder(resp.Body).Decode(&baseInfo)
	if e != nil {
		utils.ErrorLog(danmaku.Iqiyi, e.Error())
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
					utils.ErrorLog(danmaku.Iqiyi, fmt.Sprintf("%d scrape segment %d error: %s", tvId, t.segment, err.Error()))
					continue
				}
				if len(data) <= 0 {
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
	defer utils.SafeClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
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
				Mode:        danmaku.NormalMode,
			})
		}
	}

	return result, nil
}

func (c *client) Media(id string) (*danmaku.Media, error) {
	if tvIdBytes, err := base64.StdEncoding.DecodeString(id); err == nil {
		tvId, _ := strconv.ParseInt(string(tvIdBytes), 10, 64)
		baseInfo, e := c.videoBaseInfo(tvId)
		if e != nil {
			return nil, e
		}
		if !baseInfo.success() {
			return nil, fmt.Errorf("get base info fail: %s", baseInfo.Code)
		}
		media := &danmaku.Media{
			Id:       id,
			Title:    baseInfo.Data.Name,
			Cover:    baseInfo.Data.ImageUrl,
			Platform: danmaku.Iqiyi,
			Type:     danmaku.Movie,
			Desc:     baseInfo.Data.Description,
			TypeDesc: "电影",
			Episodes: []*danmaku.MediaEpisode{
				{
					Id:        id,
					Title:     baseInfo.Data.Name,
					EpisodeId: "1",
				},
			},
		}
		return media, nil
	}

	nowTime := time.Now().UnixMilli()
	params := url.Values{
		"album_id":    {id},
		"timestamp":   {strconv.FormatInt(nowTime, 10)},
		"src":         {"lw"},
		"user_id":     {""},
		"vip_status":  {"0"},
		"vip_type":    {"-1"},
		"auth_cookie": {""},
		"device_id":   {"72ce31e05a23b91ad92d36554614ec88"},
		"app_version": {"13.111.23635"},
		"scale":       {"200"},
	}
	params.Set("sign", c.sign(params))

	api := "https://mesh.if.iqiyi.com/tvg/v2/selector?" + params.Encode()
	req, _ := http.NewRequest(http.MethodGet, api, nil)
	resp, err := c.common.DoReq(req)
	if err != nil {
		return nil, err
	}
	defer utils.SafeClose(resp.Body)

	var album AlbumInfoResult
	err = json.NewDecoder(resp.Body).Decode(&album)
	if err != nil {
		return nil, err
	}
	if album.StatusCode != 0 {
		return nil, fmt.Errorf("error: %d %s %s", album.StatusCode, album.Msg, id)
	}
	if len(album.Data.Videos.FeaturePaged) < 1 {
		return nil, fmt.Errorf("%s no videos", id)
	}

	var eps = make([]*danmaku.MediaEpisode, 0, len(album.Data.Videos.FeaturePaged))
	var baseInfo *VideoBaseInfoResult
	for _, epValues := range album.Data.Videos.FeaturePaged {
		for _, ep := range epValues {
			epMatches := tvIdRegex.FindStringSubmatch(ep.PlayUrl)
			if len(epMatches) < 2 {
				continue
			}
			// 过滤掉预告
			if ep.PageUrl == "" {
				continue
			}
			// 过滤花絮
			if ep.AlbumOrder >= 1000000 {
				continue
			}
			if danmaku.InvalidEpTitle(ep.Title) {
				continue
			}
			if baseInfo == nil {
				tvId, _ := strconv.ParseInt(epMatches[1], 10, 64)
				baseInfo, _ = c.videoBaseInfo(tvId)
			}
			eps = append(eps, &danmaku.MediaEpisode{
				Id:        epMatches[1],
				EpisodeId: strconv.FormatInt(int64(ep.AlbumOrder), 10),
				Title:     ep.Title,
			})
		}
	}

	if baseInfo == nil || !baseInfo.success() {
		return nil, fmt.Errorf("%s fail to get album info", id)
	}

	result := &danmaku.Media{
		Title:    baseInfo.Data.AlbumName,
		Type:     danmaku.Series,
		TypeDesc: "剧集",
		Id:       id,
		Desc:     baseInfo.Data.Description,
		Cover:    baseInfo.Data.AlbumImageUrl,
		Episodes: eps,
		Platform: danmaku.Iqiyi,
	}

	return result, nil
}
