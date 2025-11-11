package youku

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
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

func (c *client) videoInfo(vid string) (*VideoInfoFromHtml, *ShowInfoFromHtml, error) {
	videoUrl := fmt.Sprintf("https://v.youku.com/v_show/id_%s.html", vid)
	req, err := http.NewRequest(http.MethodGet, videoUrl, nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.common.DoReq(req)
	if err != nil {
		return nil, nil, err
	}
	defer utils.SafeClose(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	htmlContent := string(bodyBytes)

	// show info
	showMatches := videoRegex.FindStringSubmatch(htmlContent)
	if len(showMatches) < 2 {
		return nil, nil, fmt.Errorf("%s match show info json fail from html", vid)
	}
	var showInfo ShowInfoFromHtml
	err = json.Unmarshal([]byte(showMatches[1]), &showInfo)
	if err != nil {
		return nil, nil, err
	}

	// base info
	matches := pageRegex.FindStringSubmatch(htmlContent)
	if len(matches) < 2 {
		return nil, nil, fmt.Errorf("%s match json fail from html", vid)
	}
	var info VideoInfoFromHtml
	err = json.Unmarshal([]byte(matches[1]), &info)
	if err != nil {
		return nil, nil, err
	}

	return &info, &showInfo, nil
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
					utils.ErrorLog(danmaku.Youku, fmt.Sprintf("%s scrape segment %d error: %s", t.vid, t.segment, e.Error()))
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
	info, _, err := c.videoInfo(vid)
	if err != nil {
		utils.ErrorLog(danmaku.Youku, fmt.Sprintf("%s video info error", err.Error()))
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
	danmaku.WriteFile(danmaku.Youku, serializer, path, filename)
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
	defer utils.SafeClose(resp.Body)

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
	defer utils.SafeClose(resp.Body)
	location := resp.Header.Get("Location")
	// /v_show/id_XNjM2OTM4MjY0NA==.html?s=ecba3364afbe46aaa122
	matches := matchVIDRegex.FindStringSubmatch(location)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (c *client) Media(showId string) (*danmaku.Media, error) {
	vid := c.getVID(showId)
	if vid == "" {
		return nil, fmt.Errorf("%s show get vid fail", showId)
	}
	baseInfo, showInfo, err := c.videoInfo(vid)
	if err != nil {
		return nil, fmt.Errorf("%s show get video info fail", showId)
	}

	var eps []*danmaku.MediaEpisode
moduleLoop:
	for _, module := range showInfo.ModuleList {
		if module.Type != 10001 {
			continue
		}
		for _, component := range module.Components {
			if component.ItemList == nil {
				continue
			}
			// 10013剧集 10311电影
			// 电影不同语言版本的showId相同 字符vid相同，只有数字vid是不同的
			if component.Type != 10013 && component.Type != 10311 {
				continue
			}
			for _, ep := range component.ItemList {
				if danmaku.InvalidEpTitle(ep.Title) {
					continue
				}
				// 可能有广告
				if ep.Mark.Text == "推荐" {
					continue
				}
				eps = append(eps, &danmaku.MediaEpisode{
					Title:     ep.Title,
					EpisodeId: strconv.FormatInt(int64(ep.StageIndex), 10),
					Id:        ep.ActionValue,
				})
			}

			break moduleLoop
		}
	}

	media := &danmaku.Media{
		Id:       baseInfo.ShowId,
		Title:    baseInfo.ShowName,
		TypeDesc: showInfo.PageMap.Extra.ShowCategory,
		Cover:    showInfo.PageMap.Extra.ShowImgV,
		Platform: danmaku.Youku,
		Episodes: eps,
	}

	return media, nil
}
