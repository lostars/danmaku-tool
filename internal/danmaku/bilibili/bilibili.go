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
	"time"

	"google.golang.org/protobuf/proto"
)

type Client struct {
	MaxWorker  int
	Cookie     string
	HttpClient *http.Client
	SavePath   string

	// 非并发安全 单线程下载每个视频弹幕 一旦并发下载这里会出问题
	// 存储的是单个视频的弹幕数据
	danmaku     []*DanmakuElem
	danmakuLock sync.Mutex
}

func (c *Client) Parse() (*danmaku.DanDanXML, error) {
	if c.danmaku == nil {
		return nil, danmaku.NewError(c, "danmaku is nil")
	}

	// TODO 合并重复弹幕 合并1s内的重复弹幕

	var data = make([]danmaku.DanDanXMLDanmaku, len(c.danmaku))
	// <d p="2.603,1,25,16777215,[bilibili]">看看 X2</d>
	// 第几秒/弹幕类型/字体大小/颜色
	for i, v := range c.danmaku {
		var attr = []string{
			strconv.FormatFloat(float64(v.Progress)/1000, 'f', 3, 64),
			strconv.FormatInt(int64(v.Mode), 10),
			strconv.FormatInt(int64(v.Fontsize), 10),
			strconv.FormatInt(int64(v.Color), 10),
			fmt.Sprintf("[%s]", c.Platform()),
		}
		d := danmaku.DanDanXMLDanmaku{
			Attributes: strings.Join(attr, ","),
			Content:    v.Content,
		}
		data[i] = d
	}

	xml := danmaku.DanDanXML{
		ChatServer:     "comment.bilibili.com",
		ChatID:         1, // TODO
		Mission:        0,
		MaxLimit:       2000,
		Source:         "k-v",
		SourceProvider: c.Platform(),
		DataSize:       len(c.danmaku),
		Danmaku:        data,
	}

	return &xml, nil
}

func (c *Client) Platform() string {
	return "bilibili"
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
		danmaku.Debugger(c).Printf("Failed to create request: %v\n", err)
		return nil
	}

	// 2. 【关键】设置 Accept-Encoding: gzip，告诉服务器客户端支持 Gzip 压缩
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Cookie", c.Cookie)

	resp, err := client.Do(req)
	if err != nil {
		danmaku.Debugger(c).Printf("HTTP request failed: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		danmaku.Debugger(c).Printf("HTTP error: %s\n", resp.Status)
		return nil
	}

	// 没有权限会返回json 400错误，但是status=200
	var contentType = resp.Header.Get("Content-Type")
	if contentType != "application/octet-stream" {
		if contentType == "application/json" {
			var raw = json.RawMessage{}
			err = json.NewDecoder(resp.Body).Decode(&raw)
			if err != nil {
				danmaku.Debugger(c).Printf("%v\n", err)
			} else {
				danmaku.Debugger(c).Printf("%s\n", string(raw))
			}
		} else {
			danmaku.Debugger(c).Printf("unknown content type: %s\n", contentType)
		}
		return nil
	}

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		danmaku.Debugger(c).Printf("Failed to create gzip reader: %v\n", err)
	}
	defer gzipReader.Close()
	reply := &DmSegMobileReply{}
	jsonBytes, err := io.ReadAll(gzipReader)
	if err != nil {
		danmaku.Debugger(c).Printf("%v\n", err)
		return nil
	}
	if err := proto.Unmarshal(jsonBytes, reply); err != nil {
		danmaku.Debugger(c).Printf("%v\n", err)
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
		return danmaku.NewError(c, "nil params")
	}
	v, ok := id.(string)
	if !ok {
		return danmaku.NewError(c, "invalid params")
	}
	realId := strings.TrimSpace(v)
	danmaku.Debugger(c).Printf("scrape id: %s\n", realId)
	if realId == "" {
		return danmaku.NewError(c, "invalid params")
	}

	// 比如 悠哉日常大王 第三季 就是一个单独的剧集 md28231846:ss36204
	//https://api.bilibili.com/pgc/view/web/season?ep_id=2231363 or season_id=12334
	params := url.Values{}
	var isEP, isSeason bool
	if strings.HasPrefix(realId, "ep") {
		isEP = true
		params.Add("ep_id", strings.Replace(realId, "ep", "", 1))
	}
	if strings.HasPrefix(realId, "ss") {
		isSeason = true
		params.Add("season_id", strings.Replace(realId, "ss", "", 1))
	}
	if len(params) == 0 {
		return danmaku.NewError(c, "only support epid or ssid")
	}

	api := "https://api.bilibili.com/pgc/view/web/season?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return danmaku.NewError(c, fmt.Sprintf("create season request err: %s", err.Error()))
	}
	req.Header.Set("Cookie", c.Cookie)
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return danmaku.NewError(c, fmt.Sprintf("get season err: %s", err.Error()))
	}
	defer resp.Body.Close()

	var series SeriesInfo
	err = json.NewDecoder(resp.Body).Decode(&series)
	if err != nil {
		return danmaku.NewError(c, fmt.Sprintf("decode season resp err: %s", err.Error()))
	}
	if series.Code != 0 {
		return danmaku.NewError(c, fmt.Sprintf("season resp error code: %v, message: %s", series.Code, series.Message))
	}

	// savePath/{platform}/{ss1234}/{index}_{epid}.xml : ./bilibili/ss1234/1_ss1234
	path := filepath.Join(c.SavePath, c.Platform(), "ss"+strconv.FormatInt(series.Result.SeasonId, 10))

	// 顺序抓取每个ep的弹幕，并发抓取每个ep弹幕
	for _, ep := range series.Result.Episodes {

		// 如果是ep则只抓取对应一集弹幕
		if isEP && "ep"+strconv.FormatInt(ep.EPId, 10) != realId {
			continue
		}

		// 排除掉预告，b站会把预告也放入其中
		if ep.SectionType == 1 {
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
					c.danmakuLock.Lock()
					c.danmaku = append(c.danmaku, data...)
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

		// 此时的标题是真实的剧集id 也可能是 "正片"
		var index = "1"
		_, err = strconv.ParseInt(ep.Title, 10, 64)
		if err == nil {
			index = ep.Title
		}
		filename := index + "_" + strconv.FormatInt(ep.EPId, 10)
		generator := danmaku.DanDanXMLGenerator{
			Indent:   true,
			Parser:   c,
			FullPath: path,
			Filename: filename,
		}
		err = generator.WriteToFile()
		if err != nil {
			if isEP {
				return danmaku.NewError(c, fmt.Sprintf("scrape ep%v wirte to file err: %s", ep.EPId, err.Error()))
			}
			if isSeason {
				// TODO anything else to do?
				danmaku.Debugger(c).Printf("scrape ep%v wirte to file err: %s", ep.EPId, err.Error())
				continue
			}
		}
		danmaku.Debugger(c).Printf("ep%v danmaku scraped done, size: %v\n", ep.EPId, len(c.danmaku))
	}

	danmaku.Debugger(c).Printf("danmaku scraped done: %s\n", series.Result.Title)

	return nil
}

type task struct {
	cid     int64
	segment int64
}

func init() {
	global := config.GetConfig()
	client := Client{
		Cookie:     global.Bilibili.Cookie,
		SavePath:   global.SavePath,
		MaxWorker:  global.Bilibili.MaxWorker,
		HttpClient: &http.Client{Timeout: time.Duration(global.Bilibili.Timeout * 1e9)},
	}
	err := danmaku.RegisterPlatform(&client)
	if err != nil {
		danmaku.Debugger(&client).Printf("%v\n", err)
	}
}
