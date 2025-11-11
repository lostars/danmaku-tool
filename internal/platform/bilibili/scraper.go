package bilibili

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
	"fmt"
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
		return err
	}

	utils.InfoLog(danmaku.Bilibili, "scrape start", "id", realId)
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
			utils.DebugLog(danmaku.Bilibili, "scrape skipped because of section type of 1", "epId", ep.EPId)
			continue
		}

		data, err := c.GetDanmaku(strconv.FormatInt(ep.EPId, 10))
		if err != nil {
			utils.InfoLog(danmaku.Bilibili, fmt.Sprintf("%d scrape error: %s", ep.EPId, err.Error()))
			continue
		}
		epTitle = ep.Title

		serializer := &danmaku.SerializerData{
			EpisodeId:       strconv.FormatInt(ep.EPId, 10),
			SeasonId:        strconv.FormatInt(series.Result.SeasonId, 10),
			DurationInMills: ep.Duration,
			Data:            data,
			ResX:            ep.Dimension.Width,
			ResY:            ep.Dimension.Height,
		}

		filename := strconv.FormatInt(ep.EPId, 10)
		danmaku.WriteFile(danmaku.Bilibili, serializer, savePath, filename)

		utils.InfoLog(danmaku.Bilibili, "ep scraped done", "epId", ep.EPId, "size", len(data))
	}

	var t = series.Result.Title
	if isEP {
		t += epTitle
	}
	utils.InfoLog(danmaku.Bilibili, "danmaku scraped done", "title", t)

	return nil
}

func (c *client) Match(param danmaku.MatchParam) ([]*danmaku.Media, error) {
	keyword := param.Title
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
		utils.InfoLog(danmaku.Bilibili, "search no result", "keyword", keyword)
		return data, nil
	}

	for _, bangumi := range result.Data.Result {
		year := time.Unix(bangumi.PubTime, 0).Year()
		if !param.MatchYear(year) {
			continue
		}

		match := param.MatchTitle(bangumi.Title)
		utils.DebugLog(danmaku.Bilibili, fmt.Sprintf("[%s] match [%s]: %v", bangumi.Title, param.Title, match))
		if !match {
			continue
		}
		clearTitle := danmaku.ClearTitle(bangumi.Title)

		var eps []*danmaku.MediaEpisode
		// 分两类处理
		/*
			1. 有EP信息 可能是剧集 也可能是电影不同的语言或配音版本
				只要不是电影类型都按照剧集处理
			2. 无EP信息 从url解析epId 则只可能是电影一类单视频
		*/
		if len(bangumi.EPs) > 0 {
			if isSeries(bangumi.MediaType) {
				for i, ep := range bangumi.EPs {
					// 如果发现 ep.Title 不是从1开始，常见的就是 第二季 36集 开始计数
					// 则从数组下标开始计数
					if danmaku.InvalidEpTitle(ep.Title) {
						continue
					}
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
				for i, v := range bangumi.EPs {
					ep := &danmaku.MediaEpisode{
						Id:        strconv.FormatInt(v.Id, 10),
						EpisodeId: strconv.FormatInt(int64(i), 10),
						Title:     v.Title,
					}
					eps = append(eps, ep)
				}
			}

		} else {
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
		}

		b := &danmaku.Media{
			Id:       strconv.FormatInt(bangumi.SeasonId, 10),
			Type:     parseMediaType(bangumi.MediaType),
			TypeDesc: bangumi.SeasonTypeName,
			Desc:     bangumi.Desc,
			Title:    clearTitle,
			Cover:    bangumi.Cover,
			Episodes: eps,
			PubTime:  bangumi.PubTime,
			Year:     year,
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

	utils.InfoLog(danmaku.Bilibili, "get danmaku done", "size", len(result))

	return result, nil
}
