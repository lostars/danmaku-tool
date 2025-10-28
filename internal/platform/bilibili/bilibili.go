package bilibili

import (
	"compress/gzip"
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

	"google.golang.org/protobuf/proto"
)

func (c *client) Init() error {
	common, err := danmaku.InitPlatformClient(danmaku.Bilibili)
	if err != nil {
		return err
	}
	c.common = common
	danmaku.RegisterScraper(c)
	danmaku.RegisterMediaSearcher(c)
	return nil
}

func init() {
	danmaku.RegisterInitializer(&client{})
}

type client struct {
	common *danmaku.PlatformClient

	// 接口签名token信息
	token tokenKey
}

func (c *client) Platform() danmaku.Platform {
	return danmaku.Bilibili
}

func (c *client) scrape(oid, pid, segmentIndex int64) []*DanmakuElem {
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
		c.common.Logger.Info(fmt.Sprintf("create request error: %s", err))
		return nil
	}

	// 2. 【关键】设置 Accept-Encoding: gzip，告诉服务器客户端支持 Gzip 压缩
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Cookie", c.common.Cookie)

	resp, err := client.Do(req)
	if err != nil {
		c.common.Logger.Info(fmt.Sprintf("request failed: %s", err))
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.common.Logger.Info("request not ok", "code", resp.Status)
		return nil
	}

	// 没有权限会返回json 400错误，但是status=200
	var contentType = resp.Header.Get("Content-Type")
	if contentType != "application/octet-stream" {
		if contentType == "application/json" {
			var raw = json.RawMessage{}
			err = json.NewDecoder(resp.Body).Decode(&raw)
			if err != nil {
				c.common.Logger.Error(err.Error())
			} else {
				c.common.Logger.Error(fmt.Sprintf("unknown error: %s", string(raw)))
			}
		} else {
			c.common.Logger.Error(fmt.Sprintf("unknown content type: %s", contentType))
		}
		return nil
	}

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		c.common.Logger.Error(fmt.Sprintf("failed to create gzip reader: %v", err))
		return nil
	}
	defer gzipReader.Close()
	reply := &DmSegMobileReply{}
	jsonBytes, err := io.ReadAll(gzipReader)
	if err != nil {
		c.common.Logger.Error(err.Error())
		return nil
	}
	if err := proto.Unmarshal(jsonBytes, reply); err != nil {
		c.common.Logger.Error(err.Error())
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

func (c *client) Scrape(id interface{}) error {
	if id == nil {
		return danmaku.PlatformError(danmaku.Bilibili, "nil params")
	}
	v, ok := id.(string)
	if !ok {
		return danmaku.PlatformError(danmaku.Bilibili, "invalid params")
	}
	realId := strings.TrimSpace(v)
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
	req.Header.Set("Cookie", c.common.Cookie)
	resp, err := c.common.HttpClient.Do(req)
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

	c.common.Logger.Info("scrape start", "id", realId)
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
			c.common.Logger.Debug("scrape skipped because of section type of 1", "epId", ep.EPId)
			continue
		}

		var videoDuration = ep.Duration/1000 + 1 // in seconds
		var segments int64
		if videoDuration%360 == 0 {
			segments = videoDuration / 360
		} else {
			segments = videoDuration/360 + 1
		}

		parser := &xmlParser{
			epId:       ep.EPId,
			seasonId:   series.Result.SeasonId,
			epDuration: ep.Duration,
		}
		if isEP {
			epTitle = ep.Title
		}
		tasks := make(chan task, segments)
		lock := sync.Mutex{}
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
					parser.danmaku = append(parser.danmaku, standardData...)
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

		filename := strconv.FormatInt(ep.EPId, 10)
		if e := c.common.XmlPersist.WriteToFile(parser, path, filename); e != nil {
			c.common.Logger.Error(e.Error())
		}

		c.common.Logger.Info("ep scraped done", "epId", ep.EPId, "size", len(parser.danmaku))
	}

	var t = series.Result.Title
	if isEP {
		t += epTitle
	}
	c.common.Logger.Info("danmaku scraped done", "title", t)

	return nil
}

type task struct {
	cid     int64
	segment int64
}
