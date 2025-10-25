package bilibili

import (
	"danmu-tool/internal/danmaku"
	"danmu-tool/internal/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

type SearchResult struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Result []struct {
			ResultType string `json:"result_type"` // media_bangumi 剧集  media_ft 电影一类
			Data       []struct {
				Type           string `json:"type"`             // 和 result_type 一致
				MediaId        int64  `json:"media_id"`         // md id
				SeasonId       int64  `json:"season_id"`        // ss id
				Cover          string `json:"cover"`            // 封面url
				SeasonTypeName string `json:"season_type_name"` // 国创/电影
				Title          string `json:"title"`            // 注意有html标签 <em class=\"keyword\">凡人</em>修仙传
				Desc           string `json:"desc"`
				EPSize         int    `json:"ep_size"`
				EPs            []struct {
					Id         int64  `json:"id"`
					Title      string `json:"title"`       // 第几集 13
					IndexTitle string `json:"index_title"` // 和 title 一样？
					LongTitle  string `json:"long_title"`  // 初入星海11
				} `json:"eps"` // 完整数据
			} `json:"data"`
		} `json:"result"`
	} `json:"data"`
}

func (c *Client) Search(keyword string) ([]*danmaku.Media, error) {
	api := "https://api.bilibili.com/x/web-interface/wbi/search/all/v2?keyword=" + keyword
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return nil, danmaku.PlatformError(danmaku.Bilibili, err.Error())
	}

	req.Header.Set("Cookie", c.Cookie)
	// TODO 添加合法ua
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36")
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, danmaku.PlatformError(danmaku.Bilibili, err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, danmaku.PlatformError(danmaku.Bilibili, fmt.Sprintf("search status: %s", resp.Status))
	}

	var result SearchResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, danmaku.PlatformError(danmaku.Bilibili, fmt.Sprintf("search parse json error: %v", err))
	}
	if result.Code != 0 {
		return nil, danmaku.PlatformError(danmaku.Bilibili, fmt.Sprintf("search result: %v %s", result.Code, result.Message))
	}
	var data = make([]*danmaku.Media, 0, 10)
	if result.Data.Result == nil || len(result.Data.Result) <= 0 {
		logger.Warn("search no result", "keyword", keyword)
		return data, nil
	}

	for _, m := range result.Data.Result {
		if m.Data == nil || len(m.Data) <= 0 {
			continue
		}

		switch m.ResultType {
		case "media_bangumi":

			for _, bangumi := range m.Data {
				var clearTitle = utils.StripHTMLTags(bangumi.Title)
				// 搜索命中的标题都带有html em标签
				if clearTitle == bangumi.Title {
					logger.Debug("search keyword bangumi skipped", "title", bangumi.Title, "resultType", m.ResultType)
					continue
				}
				var eps []*danmaku.MediaEpisode
				if bangumi.EPs != nil {
					eps = make([]*danmaku.MediaEpisode, 0, len(bangumi.EPs))
					for _, ep := range bangumi.EPs {
						eps = append(eps, &danmaku.MediaEpisode{
							Id:        strconv.FormatInt(ep.Id, 10),
							EpisodeId: ep.Title,
							Title:     ep.LongTitle,
						})
					}
				}

				b := &danmaku.Media{
					Id:       strconv.FormatInt(bangumi.SeasonId, 10),
					Type:     danmaku.Series,
					TypeDesc: bangumi.SeasonTypeName,
					Desc:     bangumi.Desc,
					Title:    clearTitle,
					Episodes: eps,
				}
				data = append(data, b)
			}

		case "media_ft":
			for _, bangumi := range m.Data {
				var clearTitle = utils.StripHTMLTags(bangumi.Title)
				// 搜索命中的标题都带有html em标签
				if clearTitle == bangumi.Title {
					logger.Debug("search keyword bangumi skipped", "title", bangumi.Title, "resultType", m.ResultType)
					continue
				}
				b := &danmaku.Media{
					Id:       strconv.FormatInt(bangumi.SeasonId, 10),
					Type:     danmaku.Movie,
					TypeDesc: bangumi.SeasonTypeName,
					Desc:     bangumi.Desc,
					Title:    clearTitle,
				}
				data = append(data, b)
			}
		}
	}

	return data, nil
}

func (c *Client) GetDanmaku(id string) ([]*danmaku.StandardDanmaku, error) {
	s := strings.Split(id, "_")
	if len(s) != 3 {
		return nil, danmaku.PlatformError(danmaku.Bilibili, "invalid id")
	}
	_, err := strconv.ParseInt(s[1], 10, 64)
	if err != nil {
		return nil, danmaku.PlatformError(danmaku.Bilibili, "invalid id")
	}
	_, err = strconv.ParseInt(s[2], 10, 64)
	if err != nil {
		return nil, danmaku.PlatformError(danmaku.Bilibili, "invalid id")
	}

	var realId = s[2]
	params := url.Values{
		"ep_id": {realId},
	}

	api := "https://api.bilibili.com/pgc/view/web/season?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return nil, danmaku.PlatformError(danmaku.Bilibili, fmt.Sprintf("create season request err: %s", err.Error()))
	}
	req.Header.Set("Cookie", c.Cookie)
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, danmaku.PlatformError(danmaku.Bilibili, fmt.Sprintf("get season err: %s", err.Error()))
	}
	defer resp.Body.Close()

	var series SeriesInfo
	err = json.NewDecoder(resp.Body).Decode(&series)
	if err != nil {
		return nil, danmaku.PlatformError(danmaku.Bilibili, fmt.Sprintf("decode season resp err: %s", err.Error()))
	}
	if series.Code != 0 {
		return nil, danmaku.PlatformError(danmaku.Bilibili, fmt.Sprintf("season resp error code: %v, message: %s", series.Code, series.Message))
	}

	var result = make([]*danmaku.StandardDanmaku, 0, 40000)
	var lock sync.Mutex
	for _, ep := range series.Result.Episodes {
		if strconv.FormatInt(ep.EPId, 10) != realId {
			continue
		}

		var videoDuration = ep.Duration/1000 + 1 // in seconds
		var segments int64
		if videoDuration%360 == 0 {
			segments = videoDuration / 360
		} else {
			segments = videoDuration/360 + 1
		}

		tasks := make(chan task, segments)
		var wg sync.WaitGroup
		for w := 0; w < c.MaxWorker; w++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				for t := range tasks {
					data := c.scrape(t.cid, 0, t.segment)
					if data == nil {
						continue
					}
					var standardData = make([]*danmaku.StandardDanmaku, 0, len(data))
					for _, d := range data {
						standardData = append(standardData, &danmaku.StandardDanmaku{
							Content:  d.Content,
							Offset:   int64(d.Progress),
							Mode:     int(d.Mode),
							Color:    int(d.Color),
							FontSize: d.Fontsize,
						})
					}
					lock.Lock()
					result = append(result, standardData...)
					lock.Unlock()
				}
			}(w)
		}

		go func() {
			for seg := int64(1); seg <= segments; seg++ {
				tasks <- task{
					cid:     ep.CId,
					segment: seg,
				}
			}
			close(tasks)
		}()

		wg.Wait()
	}

	logger.Info("get danmaku done", "size", len(result))

	return result, nil
}

func (c *Client) SearcherType() danmaku.Platform {
	return danmaku.Bilibili
}
