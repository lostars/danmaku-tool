package bilibili

import (
	"compress/gzip"
	"danmaku-tool/internal/danmaku"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"google.golang.org/protobuf/proto"
)

type client struct {
	common *danmaku.PlatformClient

	// 接口签名token信息
	token tokenKey
}

func (c *client) Media(id string) (*danmaku.Media, error) {
	series, err := c.baseInfo("", id)
	if err != nil {
		return nil, err
	}

	var eps = make([]*danmaku.MediaEpisode, 0, len(series.Result.Episodes))
	for _, ep := range series.Result.Episodes {
		eps = append(eps, &danmaku.MediaEpisode{
			Id:        strconv.FormatInt(ep.EPId, 10),
			EpisodeId: ep.Title,
			Title:     ep.ShowTitle,
		})
	}

	result := &danmaku.Media{
		Episodes: eps,
		Id:       strconv.FormatInt(series.Result.SeasonId, 10),
		Title:    series.Result.Title,
		Desc:     series.Result.Title,
		Type:     parseMediaType(series.Result.Type),
		Platform: danmaku.Bilibili,
	}

	return result, nil
}

func (c *client) Init() error {
	common, err := danmaku.InitPlatformClient(danmaku.Bilibili)
	if err != nil {
		return err
	}
	c.common = common
	danmaku.RegisterScraper(c)
	return nil
}

func init() {
	danmaku.RegisterInitializer(&client{})
}

func (c *client) Platform() danmaku.Platform {
	return danmaku.Bilibili
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
	resp, err := c.common.DoReq(req)
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

func (c *client) baseInfo(epId string, ssId string) (*SeriesInfo, error) {
	params := url.Values{}
	if epId != "" {
		params.Add("ep_id", epId)
	}
	if ssId != "" {
		params.Add("season_id", ssId)
	}

	api := "https://api.bilibili.com/pgc/view/web/season?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return nil, fmt.Errorf("create season request err: %s", err.Error())
	}
	resp, err := c.common.DoReq(req)
	if err != nil {
		return nil, fmt.Errorf("get season err: %s", err.Error())
	}
	defer resp.Body.Close()

	var series SeriesInfo
	err = json.NewDecoder(resp.Body).Decode(&series)
	if err != nil {
		return nil, fmt.Errorf("decode season resp err: %s", err.Error())
	}
	if series.Code != 0 {
		return nil, fmt.Errorf("season resp error code: %v, message: %s", series.Code, series.Message)
	}
	return &series, nil
}

func (c *client) scrape(oid, pid, segmentIndex int64) []*DanmakuElem {
	params := url.Values{
		"type":          {"1"},
		"oid":           {strconv.FormatInt(oid, 10)},
		"pid":           {strconv.FormatInt(pid, 10)},
		"segment_index": {strconv.FormatInt(segmentIndex, 10)},
	}
	api := "https://api.bilibili.com/x/v2/dm/web/seg.so?" + params.Encode()

	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		c.common.Logger.Info(fmt.Sprintf("create request error: %s", err))
		return nil
	}

	// 2. 【关键】设置 Accept-Encoding: gzip，告诉服务器客户端支持 Gzip 压缩
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Cookie", c.common.Cookie)

	resp, err := c.common.DoReq(req)
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

type task struct {
	cid     int64
	segment int64
}
