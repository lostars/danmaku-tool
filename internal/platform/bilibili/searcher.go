package bilibili

import (
	"danmu-tool/internal/danmaku"
	"danmu-tool/internal/utils"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
)

type SearchResult struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Result []struct {
			// 5=真人剧集 2=电影 4=动画剧集 1=番剧 3=纪录片
			// 5和2 都是media_ft 4是media_bangumi 用分类接口不好一次性搜索
			MediaType      int    `json:"media_type"`
			Type           string `json:"type"`             // 这个字段难以区真人剧集和电影，都算作media_ft
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
		} `json:"result"`
	} `json:"data"`
}

func (c *client) Match(keyword string) ([]*danmaku.Media, error) {
	// b站是无法搜索 S01 季节的，只能转成中文数字才能匹配
	matches := danmaku.SeriesRegex.FindStringSubmatch(keyword)
	// 是否需要匹配第几季 >1季 才转换成汉语数字进行匹配
	//original := keyword
	var ssId = int64(-1)
	if len(matches) > 3 {
		keyword = matches[1]
		//original = matches[1]
		id, err := strconv.ParseInt(matches[2], 10, 64)
		if err == nil {
			ssId = id
			if id <= 20 && id > 1 {
				//matchSeason = true
				keyword = strings.Join([]string{matches[1], "第", danmaku.ChineseNumberSlice[ssId-1], "季"}, "")
				c.common.Logger.Info(fmt.Sprintf("real search keyword %s", keyword))
			}
		}
	}

	var data = make([]*danmaku.Media, 0, 10)
	var result SearchResult
	// 分类搜索接口 搜索类型无法区分真人剧集和电影 因为都是 media_ft 只能搜索两次
	result1, e1 := c.searchByType("media_ft", keyword)
	result2, e2 := c.searchByType("media_bangumi", keyword)
	if e1 == nil {
		result.Data.Result = append(result.Data.Result, result1.Data.Result...)
	}
	if e2 == nil {
		result.Data.Result = append(result.Data.Result, result2.Data.Result...)
	}
	if result.Data.Result == nil {
		c.common.Logger.Info("search no result", "keyword", keyword)
		return data, nil
	}

	for _, bangumi := range result.Data.Result {

		keys := danmaku.MatchKeyword.FindStringSubmatch(bangumi.Title)
		if len(keys) < 2 {
			continue
		}
		matchedKeyword := keys[1]
		if !strings.Contains(keyword, strings.ReplaceAll(matchedKeyword, " ", "")) {
			continue
		}

		var clearTitle = utils.StripHTMLTags(bangumi.Title)
		clearTitle = strings.ReplaceAll(clearTitle, " ", "")
		clearTitle = danmaku.MarkRegex.ReplaceAllLiteralString(clearTitle, "")

		// 处理语言版本
		if danmaku.MatchLanguage.MatchString(keyword) {
			// 搜索词带版本
			pureLang := danmaku.MarkRegex.ReplaceAllLiteralString(keyword, "")
			if pureLang != clearTitle {
				continue
			}
		} else {
			// 搜索词不带版本但又命中了语言关键字则过滤掉
			if danmaku.MatchLanguage.MatchString(clearTitle) {
				continue
			}
		}

		var eps []*danmaku.MediaEpisode
		// 分两类处理
		/*
			1. 有EP信息 可能是剧集 也可能是一部电影的多部
				1.1 如果带了ssId进行搜索，则按照剧集进行处理
				1.2 否则就当作一部电影的多部曲来处理
					所以如果剧集故意不带ssId进行搜索 则不会出相关数据
			2. 无EP信息 丛url解析epId 则只可能是电影一类单视频
		*/
		var mediaType danmaku.MediaType
		if bangumi.EPs != nil && len(bangumi.EPs) > 0 {
			mediaType = danmaku.Series
			if ssId >= 0 {
				if ssId == 0 {
					continue
				}
				if ssId == 1 {
					// 标题一样 或者 包含第一季
					if !(keyword == clearTitle || danmaku.MatchFirstSeason.MatchString(clearTitle)) {
						continue
					}
				}
				// 大部分剧集都是需要去匹配季信息的，标题都带了
				// TODO 有些剧集不一定带了季编号在标题上 比如 刺客伍六七 在这里就被过滤掉了
				if ssId > 1 {
					if !strings.Contains(clearTitle, "第"+danmaku.ChineseNumberSlice[ssId-1]+"季") {
						continue
					}
				}

				for i, ep := range bangumi.EPs {
					// 如果发现 ep.Title 不是从1开始，常见的就是 第二季 36集 开始计数
					// 则从数组下标开始计数
					epTitle := ep.Title
					id, e := strconv.ParseInt(epTitle, 10, 64)
					if e == nil && id > 1 {
						epTitle = strconv.FormatInt(int64(i+1), 10)
					}

					eps = append(eps, &danmaku.MediaEpisode{
						Id: strconv.FormatInt(ep.Id, 10),
						// 外部需要依靠这个字段去匹配是EP几，需要正确赋值
						EpisodeId: epTitle,
						Title:     ep.LongTitle,
					})
				}
			} else {
				for _, v := range bangumi.EPs {
					episodeId := "1"
					match := false
					// 匹配搜索版本
					if danmaku.MatchLanguage.MatchString(keyword) {
						if strings.Contains(keyword, v.Title) {
							match = true
						}
					} else {
						// 匹配原版
						if strings.Contains(v.Title, "原版") {
							match = true
						}
					}
					if match {
						ep := &danmaku.MediaEpisode{
							Id:        strconv.FormatInt(v.Id, 10),
							EpisodeId: episodeId,
							Title:     v.Title,
						}
						eps = append(eps, ep)
						break
					}
				}
			}

		} else {
			if bangumi.Url != "" {
				mediaType = danmaku.Movie
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
		}

		b := &danmaku.Media{
			Id:       strconv.FormatInt(bangumi.SeasonId, 10),
			Type:     mediaType,
			TypeDesc: bangumi.SeasonTypeName,
			Desc:     bangumi.Desc,
			Title:    clearTitle,
			Episodes: eps,
			Platform: danmaku.Bilibili,
		}
		data = append(data, b)

	}

	return data, nil
}

func (c *client) searchByType(searchType string, keyword string) (*SearchResult, error) {
	api := "https://api.bilibili.com/x/web-interface/wbi/search/type?"
	params := url.Values{
		"search_type": {searchType},
		"page":        {"1"},
		"page_size":   {"30"},
		"platform":    {"pc"},
		"highlight":   {"1"},
		"keyword":     {keyword},
	}
	params, err := c.sign(params)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, api+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Cookie", c.common.Cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36")
	resp, err := c.common.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("http status: %s", resp.Status))
	}

	var result SearchResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	if result.Code != 0 {
		return nil, errors.New(fmt.Sprintf("http result code: %v %s", result.Code, result.Message))
	}

	return &result, nil
}

func (c *client) GetDanmaku(id string) ([]*danmaku.StandardDanmaku, error) {
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
	req.Header.Set("Cookie", c.common.Cookie)
	resp, err := c.common.HttpClient.Do(req)
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
		for w := 0; w < c.common.MaxWorker; w++ {
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

	c.common.Logger.Info("get danmaku done", "size", len(result))

	return result, nil
}

func (c *client) SearcherType() danmaku.Platform {
	return danmaku.Bilibili
}
