package tencent

import (
	"bytes"
	"danmu-tool/internal/danmaku"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

func (c *client) Search(keyword string) ([]*danmaku.Media, error) {
	ssId := int64(0)
	var err error
	matches := danmaku.SeriesRegex.FindStringSubmatch(keyword)
	original := keyword
	if len(matches) > 3 {
		ssId, err = strconv.ParseInt(matches[2], 10, 64)
		if err == nil {
			original = matches[1]
			if ssId > 0 && ssId <= 20 {
				// 腾讯视频带上第几季搜索更精确 第一季也是 但是命中则不会显示 xx第一季
				keyword = strings.Join([]string{matches[1], "第", danmaku.ChineseNumberSlice[ssId-1], "季"}, "")
			} else if ssId == 0 {
				// S00
				keyword = matches[1] + "剧场版"
			}
		}
	}

	logger.Debug(fmt.Sprintf("search keyword: %s", keyword))
	searchParam := SearchParam{
		Version:    "25101301",
		ClientType: 1,
		Query:      keyword,
		PageNum:    0,
		IsPrefetch: true,
		PageSize:   30,
		QueryFrom:  102,
		NeedQc:     true,
		ExtraInfo: SearchExtraInfo{
			IsNewMarkLabel:  "1",
			MultiTerminalPc: "1",
			ThemeType:       "1",
		},
	}

	paramBytes, err := json.Marshal(searchParam)
	if err != nil {
		return nil, err
	}
	searchAPI := "https://pbaccess.video.qq.com/trpc.videosearch.mobile_search.MultiTerminalSearch/MbSearch?vversion_platform=2"
	searchReq, err := http.NewRequest(http.MethodPost, searchAPI, bytes.NewBuffer(paramBytes))
	if err != nil {
		return nil, err
	}
	c.setRequest(searchReq)

	resp, err := c.HttpClient.Do(searchReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResult SearchResult
	err = json.NewDecoder(resp.Body).Decode(&searchResult)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("search http status: %s", resp.Status))
	}
	if searchResult.Ret != 0 {
		return nil, errors.New(fmt.Sprintf("search ret code: %v %s", searchResult.Ret, searchResult.Msg))
	}
	if searchResult.Data.NormalList.ItemList == nil || len(searchResult.Data.NormalList.ItemList) <= 0 {
		return nil, errors.New(fmt.Sprintf("search empty normal list"))
	}

	var result []*danmaku.Media
	for _, v := range searchResult.Data.NormalList.ItemList {
		if tencentExcludeRegex.MatchString(v.VideoInfo.SubTitle) {
			// 命中黑名单 则代表搜索不到
			logger.Info("title in blacklist", "subTitle", v.VideoInfo.SubTitle)
			continue
		}
		switch v.VideoInfo.VideoType {
		// 电影
		case 1:
			// 去掉标点之后 直接比较
			plainTitle := danmaku.MarkRegex.ReplaceAllLiteralString(v.VideoInfo.Title, "")
			plainKeyword := danmaku.MarkRegex.ReplaceAllLiteralString(v.VideoInfo.Title, "")
			if plainTitle != plainKeyword {
				continue
			}

			media, e := v.toMedia(c, danmaku.Movie, -1)
			if e != nil {
				continue
			}
			result = append(result, media)
		// 综艺
		case 10:
		// 2剧集 3动漫
		case 2, 3:
			// 匹配标题 搜出来即是命中
			if ssId == 1 && original != v.VideoInfo.Title {
				continue
			}
			if ssId > 1 && keyword != v.VideoInfo.Title {
				continue
			}
			if v.VideoInfo.SubjectDoc.VideoNum <= 0 {
				// 没有集数信息
				continue
			}
			// 搜索剧集列表
			media, e := v.toMedia(c, danmaku.Series, ssId)
			if e != nil {
				continue
			}
			result = append(result, media)
		}
	}

	return result, nil
}

func (v *SearchResultItem) toMedia(c *client, mediaType danmaku.MediaType, ssId int64) (*danmaku.Media, error) {
	seriesItems, e := c.series(v.Doc.Id)
	if e != nil {
		return nil, e
	}

	var eps = make([]*danmaku.MediaEpisode, 0, v.VideoInfo.SubjectDoc.VideoNum)
	for _, ep := range seriesItems {
		if ep.ItemParams.IsTrailer == "1" {
			continue
		}
		// 有可能vid为空
		if ep.ItemParams.VID == "" {
			continue
		}
		eps = append(eps, &danmaku.MediaEpisode{
			Id:        ep.ItemParams.VID,
			EpisodeId: ep.ItemParams.Title,
			Title:     ep.ItemParams.CTitleOutput,
		})
	}

	// 匹配剧场版 epId 暂时使用下标作为S00的epId 最新发布的在最前面
	if ssId == 0 {
		for i, ep := range eps {
			ep.EpisodeId = strconv.FormatInt(int64(len(eps)-i), 10)
		}
	}

	media := &danmaku.Media{
		Id:       v.Doc.Id,
		Type:     mediaType,
		TypeDesc: v.VideoInfo.TypeName,
		Desc:     v.VideoInfo.Desc,
		Title:    v.VideoInfo.Title,
		Episodes: eps,
		Platform: danmaku.Tencent,
	}
	return media, nil
}

var tencentExcludeRegex = regexp.MustCompile("全网搜")

func (c *client) GetDanmaku(id string) ([]*danmaku.StandardDanmaku, error) {
	// [platform]_[id]_[id]
	s := strings.Split(id, "_")
	if len(s) != 3 {
		return nil, danmaku.PlatformError(danmaku.Tencent, "invalid id")
	}
	return c.getDanmakuByVid(s[2])
}

func (c *client) SearcherType() danmaku.Platform {
	return danmaku.Tencent
}
