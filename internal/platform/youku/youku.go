package youku

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type client struct {
	common               *danmaku.PlatformClient
	token, tokenEnc, cna string
	tkLastUpdate         time.Time
}

func init() {
	danmaku.RegisterInitializer(&client{})
}

func (c *client) Init() error {
	common, err := danmaku.InitPlatformClient(danmaku.Youku)
	if err != nil {
		return err
	}
	c.common = common
	danmaku.RegisterScraper(c)
	danmaku.RegisterSerializer(danmaku.Youku, &xmlSerializer{})
	return nil
}

func (c *client) Platform() danmaku.Platform {
	return danmaku.Youku
}

/*
	优酷的视频url格式  链接中带视频vid
	https://v.youku.com/v_show/id_XMTA3MDAzODEy.html?s=cc07361a962411de83b1
	XMTA3MDAzODEy 是vid, cc07361a962411de83b1 则是 show_id
	show_id则是从视频页面 window.__PAGE_CONF__ 获取，是一个json结构
*/

func (c *client) videoInfo(vid string) (*VideoInfoFromHtml, error) {
	videoUrl := fmt.Sprintf("https://v.youku.com/v_show/id_%s.html", vid)
	req, err := http.NewRequest(http.MethodGet, videoUrl, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.common.DoReq(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	htmlContent := string(bodyBytes)
	matches := pageRegex.FindStringSubmatch(htmlContent)
	if len(matches) < 2 {
		return nil, fmt.Errorf("%s match json fail from html", vid)
	}

	var info VideoInfoFromHtml
	err = json.Unmarshal([]byte(matches[1]), &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *client) scrapeDanmaku(vid string, segmentsLen int) []*danmaku.StandardDanmaku {

	var result []*danmaku.StandardDanmaku
	tasks := make(chan task, segmentsLen)
	// 刷新token
	c.refreshToken()
	lock := sync.Mutex{}
	var wg sync.WaitGroup
	for w := 0; w < c.common.MaxWorker; w++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for t := range tasks {
				data, e := c.scrape(t.vid, t.segment)
				if e != nil {
					c.common.Logger.Error(fmt.Sprintf("%s scrape segment %d error: %s", t.vid, t.segment, e.Error()))
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
				vid:     vid,
				segment: i,
			}
		}
		close(tasks)
	}()

	wg.Wait()
	return result
}

func (c *client) scrapeVideo(vid string) {
	info, err := c.videoInfo(vid)
	if err != nil {
		c.common.Logger.Error(fmt.Sprintf("%s video info error", err.Error()))
		return
	}

	durationInSeconds, err := strconv.ParseFloat(info.Seconds, 64)
	if err != nil {
		return
	}
	// 1分钟分片
	segmentsLen := int(durationInSeconds/60 + 1)

	var result = c.scrapeDanmaku(vid, segmentsLen)

	serializer := &danmaku.SerializerData{
		EpisodeId:       vid,
		Data:            result,
		DurationInMills: int64(durationInSeconds * 1000),
	}

	path := filepath.Join(config.GetConfig().SavePath, danmaku.Youku, info.ShowId)
	title := ""
	epId, err := strconv.ParseInt(info.ShowVideoStage, 10, 64)
	if err == nil && epId > 0 {
		title = strconv.FormatInt(epId, 10) + "_"
	}
	filename := title + vid
	if e := danmaku.WriteFile(danmaku.Youku, serializer, path, filename); e != nil {
		c.common.Logger.Error(e.Error())
	}

}

type task struct {
	vid     string
	segment int
}

func (c *client) scrape(vid string, segment int) ([]*danmaku.StandardDanmaku, error) {

	params := map[string]interface{}{
		"vid": vid,
		"mat": segment,
	}
	query, data := c.sign(params, danmakuList)
	fullURL := fmt.Sprintf("https://acs.youku.com/h5/%s/1.0/?%s", danmakuList.api, query.Encode())

	formData := url.Values{}
	formData.Set("data", data)
	reqBody := formData.Encode()

	req, _ := http.NewRequest(http.MethodPost, fullURL, strings.NewReader(reqBody))
	c.setReq(req)

	resp, err := c.common.DoReq(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResult APIResult
	err = json.NewDecoder(resp.Body).Decode(&apiResult)
	if err != nil {
		return nil, err
	}
	if !apiResult.success() {
		return nil, fmt.Errorf("%s", strings.Join(apiResult.Ret, "|"))
	}
	var danmakuResult DanmakuResult
	err = json.Unmarshal([]byte(apiResult.Data.Result), &danmakuResult)
	if err != nil {
		return nil, err
	}

	var result = make([]*danmaku.StandardDanmaku, 0, len(danmakuResult.Data.Result))
	for _, d := range danmakuResult.Data.Result {
		standard := &danmaku.StandardDanmaku{
			Content:     d.Content,
			Mode:        danmaku.NormalMode,
			OffsetMills: d.PlayAt,
			Platform:    danmaku.Youku,
			Color:       danmaku.WhiteColor,
			FontSize:    25,
		}
		var property DanmakuPropertyResult
		err = json.Unmarshal([]byte(d.Property), &property)
		if err == nil {
			standard.Color = property.Color
			switch property.Pos {
			case 1:
				standard.Mode = danmaku.TopMode
			case 2:
				standard.Mode = danmaku.BottomMode
			}
		}
		result = append(result, standard)
	}

	return result, nil
}

func (c *client) getVID(showId string) string {
	//	https://v.youku.com/video?s=ecba3364afbe46aaa122 会 302 到视频地址
	req, _ := http.NewRequest(http.MethodGet, "https://v.youku.com/video?s="+showId, nil)
	c.common.HttpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := c.common.DoReq(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	location := resp.Header.Get("Location")
	// /v_show/id_XNjM2OTM4MjY0NA==.html?s=ecba3364afbe46aaa122
	matches := matchVIDRegex.FindStringSubmatch(location)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
