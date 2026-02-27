package bilibili

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

func (c *client) Scrape(realId string) error {
	if strings.HasPrefix(realId, "BV1") {
		// support BV
		return c.scrapeBV(realId)
	}

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
		isEP = true
		// 默认为epId
		epId = realId
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
		if isEP && strconv.FormatInt(ep.EPId, 10) != epId {
			continue
		}

		// 排除掉预告，b站会把预告也放入其中
		if ep.SectionType == 1 {
			utils.DebugLog(danmaku.Bilibili, "scrape skipped because of section type of 1", "epId", ep.EPId)
			continue
		}

		epTitle = ep.Title
		serializer := &danmaku.SerializerData{
			EpisodeId:       strconv.FormatInt(ep.EPId, 10),
			SeasonId:        strconv.FormatInt(series.Result.SeasonId, 10),
			DurationInMills: ep.Duration,
			ResX:            ep.Dimension.Width,
			ResY:            ep.Dimension.Height,
			Platform:        danmaku.Bilibili,
			FullPath:        savePath,
			Filename:        strconv.FormatInt(ep.EPId, 10),
		}
		if err = danmaku.CheckFile(serializer); err != nil {
			utils.ErrorLog(danmaku.Bilibili, err.Error())
			continue
		}

		serializer.Data, err = c.GetDanmaku(strconv.FormatInt(ep.EPId, 10))
		if err != nil {
			utils.ErrorLog(danmaku.Bilibili, err.Error(), "epId", ep.EPId)
			continue
		}

		danmaku.WriteFile(serializer)
		utils.InfoLog(danmaku.Bilibili, "ep scraped done", "epId", ep.EPId, "size", len(serializer.Data))
	}

	var t = series.Result.Title
	if isEP {
		t += epTitle
	}
	utils.InfoLog(danmaku.Bilibili, "danmaku scraped done", "title", t)

	return nil
}

func (c *client) scrapeBV(bvid string) error {
	videoUrl := "https://www.bilibili.com/video/" + bvid + "/"

	req, err := http.NewRequest(http.MethodGet, videoUrl, nil)
	if err != nil {
		return err
	}
	resp, err := c.DoReq(req)
	if err != nil {
		return err
	}
	defer utils.SafeClose(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	htmlContent := string(bodyBytes)

	cidMatches := bvCidRegex.FindStringSubmatch(htmlContent)
	if len(cidMatches) < 2 {
		utils.ErrorLog(danmaku.Bilibili, "can't parse video cid")
		return nil
	}
	// duration in mills
	durationMatches := bvDurationRegex.FindStringSubmatch(htmlContent)
	if len(durationMatches) < 2 {
		utils.ErrorLog(danmaku.Bilibili, "can't parse video duration")
		return nil
	}
	var cid, duration = int64(0), int64(0)
	if cid, err = strconv.ParseInt(cidMatches[1], 10, 64); err != nil {
		utils.ErrorLog(danmaku.Bilibili, fmt.Sprintf("can't parse video cid: %s", cidMatches[1]))
		return nil
	}
	if duration, err = strconv.ParseInt(durationMatches[1], 10, 64); err != nil {
		utils.ErrorLog(danmaku.Bilibili, fmt.Sprintf("can't parse video duration: %s", durationMatches[1]))
		return nil
	}
	var width, height = int64(0), int64(0)
	hwMatches := bvWHRegex.FindStringSubmatch(htmlContent)
	if len(hwMatches) < 3 {
		utils.ErrorLog(danmaku.Bilibili, "can't parse video width and height")
		return nil
	}
	if width, err = strconv.ParseInt(hwMatches[1], 10, 64); err != nil {
		utils.ErrorLog(danmaku.Bilibili, fmt.Sprintf("can't parse video width: %s", hwMatches[1]))
		return nil
	}
	if height, err = strconv.ParseInt(hwMatches[2], 10, 64); err != nil {
		utils.ErrorLog(danmaku.Bilibili, fmt.Sprintf("can't parse video height: %s", hwMatches[2]))
		return nil
	}

	segments := c.videoSegments(duration)
	tasks := make(chan task, c.MaxWorker)
	ch := make(chan []*danmaku.StandardDanmaku, c.MaxWorker)
	var wg sync.WaitGroup
	for w := 0; w < c.MaxWorker; w++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for t := range tasks {
				data, e := c.scrape(t.cid, 0, t.segment)
				if e != nil {
					utils.ErrorLog(danmaku.Bilibili, e.Error(), "cid", t.cid, "segment", t.segment)
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
				ch <- standardData
			}
		}(w)
	}
	go func() {
		for seg := int64(1); seg <= segments; seg++ {
			tasks <- task{
				cid:     cid,
				segment: seg,
			}
		}
		close(tasks)
	}()
	go func() {
		wg.Wait()
		close(ch)
	}()

	// save file
	savePath := filepath.Join(config.GetConfig().SavePath, danmaku.Bilibili, bvid)
	serializer := &danmaku.SerializerData{
		EpisodeId:       bvid,
		SeasonId:        bvid,
		DurationInMills: duration,
		ResX:            int(width),
		ResY:            int(height),
		Platform:        danmaku.Bilibili,
		FullPath:        savePath,
		Filename:        bvid,
	}
	if err = danmaku.CheckFile(serializer); err != nil {
		utils.ErrorLog(danmaku.Bilibili, err.Error())
		return nil
	}
	for d := range ch {
		serializer.Data = append(serializer.Data, d...)
	}

	danmaku.WriteFile(serializer)
	utils.InfoLog(danmaku.Bilibili, "bv scraped done", "bvid", bvid, "size", len(serializer.Data))

	return nil
}

var bvCidRegex = regexp.MustCompile(`"cid":(\d+),`)
var bvDurationRegex = regexp.MustCompile(`"timelength":(\d+),`)
var bvWHRegex = regexp.MustCompile(`"width":(\d{4}),"height":(\d{4})`)

func (c *client) videoSegments(durationInMills int64) int64 {
	var videoDuration = durationInMills/1000 + 1
	var segments int64
	if videoDuration%360 == 0 {
		segments = videoDuration / 360
	} else {
		segments = videoDuration/360 + 1
	}
	return segments
}

func (c *client) Match(param danmaku.MatchParam) ([]*danmaku.Media, error) {
	keyword := param.Title
	var data = make([]*danmaku.Media, 0, 10)
	var result SearchResult
	// 分类搜索接口 搜索类型无法区分真人剧集和电影 因为都是 media_ft 只能搜索两次
	if result1, e1 := c.searchByType("media_ft", keyword); e1 == nil {
		result.Data.Result = append(result.Data.Result, result1.Data.Result...)
	}
	if result2, e2 := c.searchByType("media_bangumi", keyword); e2 == nil {
		result.Data.Result = append(result.Data.Result, result2.Data.Result...)
	}
	if result.Code != 0 {
		return data, fmt.Errorf("%d %s", result.Code, result.Message)
	}
	if len(result.Data.Result) < 1 {
		return data, nil
	}

	for _, bangumi := range result.Data.Result {
		year := time.Unix(bangumi.PubTime, 0).Year()
		mediaId := strconv.FormatInt(bangumi.SeasonId, 10)
		matchParam := danmaku.InternalMatchParam{
			Title:   bangumi.Title,
			Year:    year,
			MediaId: mediaId,
		}
		match := param.Match(matchParam)
		utils.DebugLog(danmaku.Bilibili, fmt.Sprintf("[%s] match [%s]: %v", bangumi.Title, param.Title, match))
		if !match {
			continue
		}
		clearTitle := danmaku.ClearTitle(bangumi.Title)

		var eps = make([]*danmaku.MediaEpisode, 0, 200)
		// 分两类处理
		/*
			1. 有EP信息 可能是剧集 也可能是电影不同的语言或配音版本
				只要不是电影类型都按照剧集处理
			2. 无EP信息 从url解析epId 则只可能是电影一类单视频
		*/
		if len(bangumi.EPs) > 0 {
			if isSeries(bangumi.MediaType) {
				for _, ep := range bangumi.EPs {
					if danmaku.InvalidEpTitle(ep.Title) {
						continue
					}
					// 排除预告
					if len(ep.Badges) > 0 && danmaku.InvalidEpTitle(ep.Badges[0].Text) {
						continue
					}
					eps = append(eps, &danmaku.MediaEpisode{
						Id: strconv.FormatInt(ep.Id, 10),
						// 外部需要依靠这个字段去匹配是EP几，需要正确赋值
						EpisodeId: ep.Title,
						Title:     ep.LongTitle,
					})
				}
			} else {
				for i, v := range bangumi.EPs {
					if danmaku.InvalidEpTitle(v.Title) {
						continue
					}
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

		typeStr := strconv.FormatInt(int64(bangumi.MediaType), 10)
		b := &danmaku.Media{
			Id:           mediaId,
			InternalType: typeStr,
			Desc:         bangumi.Desc,
			Title:        clearTitle,
			Cover:        bangumi.Cover,
			Episodes:     eps,
			EpisodeCount: len(eps),
			PubTime:      bangumi.PubTime,
			Year:         year,
			Platform:     danmaku.Bilibili,
		}
		b.MediaType(c)
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
	for _, ep := range series.Result.Episodes {
		if strconv.FormatInt(ep.EPId, 10) != realId {
			continue
		}

		segments := c.videoSegments(ep.Duration)
		tasks := make(chan task, c.MaxWorker)
		ch := make(chan []*danmaku.StandardDanmaku, c.MaxWorker)
		var wg sync.WaitGroup
		for w := 0; w < c.MaxWorker; w++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				for t := range tasks {
					data, e := c.scrape(t.cid, 0, t.segment)
					if e != nil {
						utils.ErrorLog(danmaku.Bilibili, e.Error(), "cid", t.cid, "segment", t.segment)
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
					ch <- standardData
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

		go func() {
			wg.Wait()
			close(ch)
		}()
		for m := range ch {
			result = append(result, m...)
		}
	}

	utils.InfoLog(danmaku.Bilibili, "get danmaku done", "size", len(result))

	return result, nil
}
