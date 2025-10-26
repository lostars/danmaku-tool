package bilibili

import (
	"danmu-tool/internal/danmaku"
	"danmu-tool/internal/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
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
				Url            string `json:"url"`              // 该字段保存的是剧集链接或者ep链接，电影可以从该url解析epid
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

var seriesRegex = regexp.MustCompile("(.*)\\sS(\\d{1,3})E(\\d{1,3})$")
var chineseVersionRegex = regexp.MustCompile("中配版|粤配版")
var chineseNumber = strings.Split("一,二,三,四,五,六,七,八,九,十,十一,十二,十三,十四,十五,十六,十七,十八,十九,二十", ",")

func (c *Client) Search(keyword string) ([]*danmaku.Media, error) {
	// b站是无法搜索 S01 季节的，只能转成中文数字才能匹配
	matches := seriesRegex.FindStringSubmatch(keyword)
	if len(matches) > 3 {
		ssId, err := strconv.ParseInt(matches[2], 10, 64)
		if err == nil && ssId <= 20 {
			keyword = strings.Join([]string{matches[1], "第", chineseNumber[ssId-1], "季"}, "")
			logger.Info(fmt.Sprintf("real search keyword %s", keyword))
		}
	}

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

	var checkChineseVersion = true
	// 本身搜索词就带了中文版本则不进行过滤
	if chineseVersionRegex.MatchString(keyword) {
		checkChineseVersion = false
	}
	var filtered []*danmaku.Media
	for _, m := range result.Data.Result {
		if m.Data == nil || len(m.Data) <= 0 {
			continue
		}

		for _, bangumi := range m.Data {
			var clearTitle = utils.StripHTMLTags(bangumi.Title)
			// 搜索命中的标题都带有html em标签 如果一样说明是广告或者推荐一类
			if clearTitle == bangumi.Title {
				logger.Debug("search keyword bangumi skipped", "title", bangumi.Title, "resultType", m.ResultType)
				continue
			}
			switch m.ResultType {
			case "media_ft":
				var eps = make([]*danmaku.MediaEpisode, 0)
				if bangumi.Url != "" {
					// https://www.bilibili.com/bangumi/play/ep747309?theme=movie
					str := path.Base(bangumi.Url)[2:]
					if strings.Contains(str, "?") {
						str = strings.Split(str, "?")[0]
					}
					ep := &danmaku.MediaEpisode{
						Id:        str,
						EpisodeId: clearTitle,
						Title:     clearTitle,
					}
					eps = append(eps, ep)
				}
				b := &danmaku.Media{
					Id:       strconv.FormatInt(bangumi.SeasonId, 10),
					Type:     danmaku.Movie,
					TypeDesc: bangumi.SeasonTypeName,
					Desc:     bangumi.Desc,
					Title:    clearTitle,
					Episodes: eps,
				}
				if checkChineseVersion && chineseVersionRegex.MatchString(clearTitle) {
					filtered = append(filtered, b)
				} else {
					data = append(data, b)
				}

			case "media_bangumi":
				var eps []*danmaku.MediaEpisode
				if bangumi.EPs != nil {
					eps = make([]*danmaku.MediaEpisode, 0, len(bangumi.EPs))
					for i, ep := range bangumi.EPs {
						// 如果发现 ep.Title 不是从1开始，常见的就是 第二季 36集 开始计数
						epTitle := ep.Title
						id, e := strconv.ParseInt(epTitle, 10, 64)
						if e == nil && id > 1 {
							epTitle = strconv.FormatInt(int64(i), 10)
						}

						eps = append(eps, &danmaku.MediaEpisode{
							Id:        strconv.FormatInt(ep.Id, 10),
							EpisodeId: epTitle,
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

				if checkChineseVersion && chineseVersionRegex.MatchString(clearTitle) {
					filtered = append(filtered, b)
				} else {
					data = append(data, b)
				}
			}
		}
	}

	if checkChineseVersion && len(data) <= 0 {
		data = append(data, filtered...)
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
