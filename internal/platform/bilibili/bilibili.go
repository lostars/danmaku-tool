package bilibili

import (
	"compress/gzip"
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"
)

type Client struct {
	MaxWorker  int
	Cookie     string
	HttpClient *http.Client

	DataPersists []danmaku.DataPersist

	// 非并发安全 单线程下载每个视频弹幕 一旦并发下载这里会出问题
	// 存储的是单个视频的弹幕数据
	danmaku        []*danmaku.StandardDanmaku
	danmakuLock    sync.Mutex
	epId, seasonId int64
	// ep时长 ms
	epDuration int64
}

func (c *Client) Parse() (*danmaku.DataXML, error) {
	if c.danmaku == nil {
		return nil, danmaku.PlatformError(danmaku.Bilibili, fmt.Sprintf("ep%v danmaku is nil", c.epId))
	}

	// 合并重复弹幕
	var source = c.danmaku
	if config.GetConfig().Bilibili.MergeDanmakuInMills > 0 {
		var merged = danmaku.MergeDanmaku(c.danmaku, config.GetConfig().Bilibili.MergeDanmakuInMills, c.epDuration)
		source = make([]*danmaku.StandardDanmaku, 0, len(merged))
		for _, v := range merged {
			source = append(source, v)
		}
	}

	var data = make([]danmaku.DataXMLDanmaku, len(source))
	// <d p="2.603,1,25,16777215,[bilibili]">看看 X2</d>
	// 第几秒/弹幕类型/字体大小/颜色
	for i, v := range source {
		var attr = []string{
			strconv.FormatFloat(float64(v.Offset)/1000, 'f', 2, 64),
			strconv.FormatInt(int64(v.Mode), 10),
			strconv.FormatInt(int64(v.FontSize), 10),
			strconv.FormatInt(int64(v.Color), 10),
			fmt.Sprintf("[%s]", c.Platform()),
		}
		d := danmaku.DataXMLDanmaku{
			Attributes: strings.Join(attr, ","),
			Content:    v.Content,
		}
		data[i] = d
	}

	xml := danmaku.DataXML{
		ChatServer:     "comment.bilibili.com",
		ChatID:         strconv.FormatInt(c.seasonId, 10) + "_" + strconv.FormatInt(c.epId, 10),
		Mission:        0,
		MaxLimit:       2000,
		Source:         "k-v",
		SourceProvider: danmaku.Bilibili,
		DataSize:       len(source),
		Danmaku:        data,
	}

	return &xml, nil
}

func (c *Client) Platform() danmaku.Platform {
	return danmaku.Bilibili
}

func (c *Client) scrape(oid, pid, segmentIndex int64) []*DanmakuElem {
	params := url.Values{
		"type":          {"1"},
		"oid":           {strconv.FormatInt(oid, 10)},
		"pid":           {strconv.FormatInt(pid, 10)},
		"segment_index": {strconv.FormatInt(segmentIndex, 10)},
	}
	api := "https://api.bilibili.com/x/v2/dm/web/seg.so?" + params.Encode()

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		logger.Info("Failed to create request", err)
		return nil
	}

	// 2. 【关键】设置 Accept-Encoding: gzip，告诉服务器客户端支持 Gzip 压缩
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Cookie", c.Cookie)

	resp, err := client.Do(req)
	if err != nil {
		logger.Info("HTTP request failed", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Info("HTTP error", "code", resp.Status)
		return nil
	}

	// 没有权限会返回json 400错误，但是status=200
	var contentType = resp.Header.Get("Content-Type")
	if contentType != "application/octet-stream" {
		if contentType == "application/json" {
			var raw = json.RawMessage{}
			err = json.NewDecoder(resp.Body).Decode(&raw)
			if err != nil {
				logger.Error(err.Error())
			} else {
				logger.Error("unknown error", "json", string(raw))
			}
		} else {
			logger.Error("unknown content type", "contentType", contentType)
		}
		return nil
	}

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		logger.Error("failed to create gzip reader", err)
		return nil
	}
	defer gzipReader.Close()
	reply := &DmSegMobileReply{}
	jsonBytes, err := io.ReadAll(gzipReader)
	if err != nil {
		logger.Error(err.Error())
		return nil
	}
	if err := proto.Unmarshal(jsonBytes, reply); err != nil {
		logger.Error(err.Error())
		return nil
	}
	return reply.GetElems()
}

type SeriesInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  struct {
		Cover string `json:"cover"`
		// 当前EP所在Season所有EPs
		Episodes []struct { // 0 第一集 1 第二集 预告可能也会在里面
			AId         int64  `json:"aid"`
			BVId        string `json:"bvid"`
			CId         int64  `json:"cid"`
			Duration    int64  `json:"duration"` // in Millisecond
			EPId        int64  `json:"ep_id"`
			SectionType int    `json:"section_type"` // 1 是预告之类的 0是正常剧集？？
			Link        string `json:"link"`
			Title       string `json:"title"`
			PubTime     int64  `json:"pub_time"`
		} `json:"episodes"`
		// 同系列所有季信息
		Seasons []struct {
			MediaId     int64  `json:"media_id"`
			SeasonId    int64  `json:"season_id"`
			SeasonType  int    `json:"season_type"`
			SeasonTitle string `json:"season_title"`
			Cover       string `json:"cover"`
		} `json:"seasons"`
		Evaluate    string   `json:"evaluate"`
		Link        string   `json:"link"`
		MediaId     int64    `json:"media_id"`
		SeasonId    int64    `json:"season_id"`
		SeasonTitle string   `json:"season_title"`
		NewEP       struct { // 最新一集信息
			Id    int64  `json:"id"`     // 最新一集epid
			IsNew int    `json:"is_new"` // 0否 1是
			Title string `json:"title"`
		} `json:"new_ep"`
		Title    string `json:"title"`
		SubTitle string `json:"subtitle"`
		Total    int    `json:"total"` // 未完结：大多为-1 已完结：正整数
		Type     int    `json:"type"`  // 1：番剧 2：电影 3：纪录片 4：国创 5：电视剧 7：综艺
	} `json:"result"`
}

func (c *Client) Scrape(id interface{}) error {
	if id == nil {
		return danmaku.PlatformError(danmaku.Bilibili, "nil params")
	}
	v, ok := id.(string)
	if !ok {
		return danmaku.PlatformError(danmaku.Bilibili, "invalid params")
	}
	realId := strings.TrimSpace(v)
	logger.Info("scrape id", realId)
	if realId == "" {
		return danmaku.PlatformError(danmaku.Bilibili, "invalid params")
	}

	// 比如 悠哉日常大王 第三季 就是一个单独的剧集 md28231846:ss36204
	//https://api.bilibili.com/pgc/view/web/season?ep_id=2231363 or season_id=12334
	params := url.Values{}
	var isEP bool
	if strings.HasPrefix(realId, "ep") {
		isEP = true
		params.Add("ep_id", strings.Replace(realId, "ep", "", 1))
	}
	if strings.HasPrefix(realId, "ss") {
		params.Add("season_id", strings.Replace(realId, "ss", "", 1))
	}
	if len(params) == 0 {
		return danmaku.PlatformError(danmaku.Bilibili, "only support epid or ssid")
	}

	api := "https://api.bilibili.com/pgc/view/web/season?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return danmaku.PlatformError(danmaku.Bilibili, fmt.Sprintf("create season request err: %s", err.Error()))
	}
	req.Header.Set("Cookie", c.Cookie)
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return danmaku.PlatformError(danmaku.Bilibili, fmt.Sprintf("get season err: %s", err.Error()))
	}
	defer resp.Body.Close()

	var series SeriesInfo
	err = json.NewDecoder(resp.Body).Decode(&series)
	if err != nil {
		return danmaku.PlatformError(danmaku.Bilibili, fmt.Sprintf("decode season resp err: %s", err.Error()))
	}
	if series.Code != 0 {
		return danmaku.PlatformError(danmaku.Bilibili, fmt.Sprintf("season resp error code: %v, message: %s", series.Code, series.Message))
	}

	// savePath/{platform}/{ssid}/{epid}.xml : ./bilibili/1234/11234
	path := filepath.Join(config.GetConfig().SavePath, danmaku.Bilibili, strconv.FormatInt(series.Result.SeasonId, 10))

	// 顺序抓取每个ep的弹幕，并发抓取每个ep弹幕
	var epTitle string
	for _, ep := range series.Result.Episodes {

		// 如果是ep则只抓取对应一集弹幕
		if isEP && "ep"+strconv.FormatInt(ep.EPId, 10) != realId {
			continue
		}

		// 排除掉预告，b站会把预告也放入其中
		if ep.SectionType == 1 {
			logger.Debug("scrape skipped because of section type of 1", "epId", ep.EPId)
			continue
		}

		var videoDuration = ep.Duration/1000 + 1 // in seconds
		var segments int64
		if videoDuration%360 == 0 {
			segments = videoDuration / 360
		} else {
			segments = videoDuration/360 + 1
		}

		c.epId = ep.EPId
		c.seasonId = series.Result.SeasonId
		c.epDuration = ep.Duration
		if isEP {
			epTitle = ep.Title
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
					c.danmakuLock.Lock()
					c.danmaku = append(c.danmaku, standardData...)
					c.danmakuLock.Unlock()
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

		filename := strconv.FormatInt(ep.EPId, 10)
		for i, persist := range c.DataPersists {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				if e := persist.WriteToFile(path, filename); e != nil {
					logger.Error(e.Error())
				}
			}(i)
		}
		wg.Wait()

		logger.Info("scraped done", "epId", ep.EPId, "size", len(c.danmaku))
	}

	var t = series.Result.Title
	if isEP {
		t += epTitle
	}
	logger.Info("danmaku scraped done", "size", t)

	return nil
}

var logger *slog.Logger

type task struct {
	cid     int64
	segment int64
}
