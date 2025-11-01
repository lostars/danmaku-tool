package bilibili

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"fmt"
	"math"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

func (c *client) Scrape(realId string) error {
	// 比如 悠哉日常大王 第三季 就是一个单独的剧集 md28231846:ss36204
	//https://api.bilibili.com/pgc/view/web/season?ep_id=2231363 or season_id=12334
	var isEP bool
	epId := ""
	ssId := ""
	if strings.HasPrefix(realId, "ep") {
		isEP = true
		epId = strings.Replace(realId, "ep", "", 1)
	}
	if strings.HasPrefix(realId, "ss") {
		ssId = strings.Replace(realId, "ss", "", 1)
	}
	if epId == "" && ssId == "" {
		return fmt.Errorf("only support epid or ssid")
	}

	series, err := c.baseInfo(epId, ssId)
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println(epId)
		fmt.Println(ssId)
		return err
	}

	c.common.Logger.Info("scrape start", "id", realId)
	// savePath/{platform}/{ssid}/{epid}.xml : ./bilibili/1234/11234
	savePath := filepath.Join(config.GetConfig().SavePath, danmaku.Bilibili, strconv.FormatInt(series.Result.SeasonId, 10))

	// 顺序抓取每个ep的弹幕，并发抓取每个ep弹幕
	var epTitle string
	for _, ep := range series.Result.Episodes {

		// 如果是ep则只抓取对应一集弹幕
		if isEP && "ep"+strconv.FormatInt(ep.EPId, 10) != realId {
			continue
		}

		// 排除掉预告，b站会把预告也放入其中
		if ep.SectionType == 1 {
			c.common.Logger.Debug("scrape skipped because of section type of 1", "epId", ep.EPId)
			continue
		}

		data, err := c.GetDanmaku(strconv.FormatInt(ep.EPId, 10))
		if err != nil {
			c.common.Logger.Info(fmt.Sprintf("%d scrape error: %s", ep.EPId, err.Error()))
			continue
		}
		epTitle = ep.Title

		serializer := &danmaku.SerializerData{
			EpisodeId:       strconv.FormatInt(ep.EPId, 10),
			SeasonId:        strconv.FormatInt(series.Result.SeasonId, 10),
			DurationInMills: ep.Duration,
			Data:            data,
		}

		filename := strconv.FormatInt(ep.EPId, 10)
		if e := danmaku.WriteFile(danmaku.Bilibili, serializer, savePath, filename); e != nil {
			c.common.Logger.Error(e.Error())
		}

		c.common.Logger.Info("ep scraped done", "epId", ep.EPId, "size", len(data))
	}

	var t = series.Result.Title
	if isEP {
		t += epTitle
	}
	c.common.Logger.Info("danmaku scraped done", "title", t)

	return nil
}

func (c *client) Match(param danmaku.MatchParam) ([]*danmaku.Media, error) {
	keyword := param.FileName
	var ssId = int64(param.SeasonId)

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
		if !param.MatchYear(time.Unix(bangumi.PubTime, 0).Year()) {
			continue
		}

		var clearTitle = danmaku.ClearTitle(bangumi.Title)

		target := keyword
		match := danmaku.Tokenizer.Match(clearTitle, target)
		c.common.Logger.Debug(fmt.Sprintf("[%s] match [%s]: %v", clearTitle, target, match))
		if !match {
			continue
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
				// 获取第一集检查时长
				if param.DurationSeconds > 0 {
					ss, err := c.baseInfo(strconv.FormatInt(bangumi.EPs[0].Id, 10), "")
					if err == nil && ss.Result.Episodes != nil {
						durationMills := ss.Result.Episodes[0].Duration
						if math.Abs(float64(durationMills/1000-param.DurationSeconds)) > 300 {
							continue
						}
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

func (c *client) GetDanmaku(realId string) ([]*danmaku.StandardDanmaku, error) {
	series, err := c.baseInfo(realId, "")
	if err != nil {
		return nil, err
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
							Content:     d.Content,
							OffsetMills: int64(d.Progress),
							Mode:        int(d.Mode),
							Color:       int(d.Color),
							FontSize:    d.Fontsize,
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
