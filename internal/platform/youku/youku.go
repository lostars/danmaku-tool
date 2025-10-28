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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type client struct {
	common          *danmaku.PlatformClient
	token, tokenEnc string
	tkLastUpdate    time.Time
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
	return nil
}

func (c *client) Platform() danmaku.Platform {
	return danmaku.Youku
}

/*
	优酷的视频url格式  链接中带视频vid
	https://v.youku.com/v_show/id_XMTA3MDAzODEy.html?s=cc07361a962411de83b1
	XMTA3MDAzODEy 是vid, cc07361a962411de83b1 则是 show_id
	show_id则是从视频页面 window.__INITIAL_DATA__ 获取，是一个json结构
*/

// var videoRegex = regexp.MustCompile(`<script>window\.__INITIAL_DATA__\s=(\{.*});</script>`)
var pageRegex = regexp.MustCompile(`<script>window\.__PAGE_CONF__\s=(\{.*});`)

func (c *client) Scrape(id interface{}) error {
	if id == nil {
		return danmaku.PlatformError(danmaku.Youku, "invalid params")
	}
	idStr, ok := id.(string)
	if !ok {
		return danmaku.PlatformError(danmaku.Youku, "invalid params")
	}

	c.scrapeVideo(idStr)

	return nil
}

func (c *client) videoInfo(vid string) (*VideoInfoFromHtml, error) {
	videoUrl := fmt.Sprintf("https://v.youku.com/v_show/id_%s.html", vid)
	req, err := http.NewRequest(http.MethodGet, videoUrl, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.common.HttpClient.Do(req)
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

func (c *client) scrapeVideo(vid string) {
	info, err := c.videoInfo(vid)
	if err != nil {
		c.common.Logger.Error(fmt.Sprintf("%s video info error", err.Error()))
		return
	}

	duration, err := strconv.ParseFloat(info.Seconds, 64)
	if err != nil {
		return
	}
	// 1分钟分片
	segmentsLen := int(duration/60 + 1)

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

	parser := &xmlParser{
		vid:      vid,
		danmaku:  result,
		duration: int64(duration) + 1,
	}

	path := filepath.Join(config.GetConfig().SavePath, danmaku.Youku, info.ShowId)
	title := ""
	epId, err := strconv.ParseInt(info.ShowVideoStage, 10, 64)
	if err == nil && epId > 0 {
		title = strconv.FormatInt(epId, 10) + "_"
	}
	filename := title + vid
	if e := c.common.XmlPersist.WriteToFile(parser, path, filename); e != nil {
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
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	req.Header.Set("cookie", fmt.Sprintf("_m_h5_tk=%s;_m_h5_tk_enc=%s", c.token, c.tokenEnc))

	resp, err := c.common.HttpClient.Do(req)
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
			Content:  d.Content,
			Mode:     1,
			Offset:   d.PlayAt,
			Platform: danmaku.Youku,
		}
		var property DanmakuPropertyResult
		err = json.Unmarshal([]byte(d.Property), &property)
		if err == nil {
			standard.Color = property.Color
			//standard.Mode = property.Pos
			//standard.FontSize = int32(property.Size)
		}
		result = append(result, standard)
	}

	return result, nil
}
