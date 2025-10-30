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

func (c *client) Match(param danmaku.MatchParam) ([]*danmaku.Media, error) {
	keyword := param.FileName
	ssId := int64(-1)
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

	c.common.Logger.Debug(fmt.Sprintf("search keyword: %s", keyword))
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

	resp, err := c.common.HttpClient.Do(searchReq)
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

	var itemList []SearchResultItem
	if searchResult.Data.NormalList.ItemList != nil {
		itemList = append(itemList, searchResult.Data.NormalList.ItemList...)
	}
	if searchResult.Data.AreaBoxList != nil {
		for _, v := range searchResult.Data.AreaBoxList {
			// 有时候从normalList不出数据，需要从areaBoxList中获取
			if v.BoxId == "MainNeed" && v.ItemList != nil {
				itemList = append(itemList, v.ItemList...)
			}
		}
	}

	var result []*danmaku.Media
	for _, v := range itemList {
		if tencentExcludeRegex.MatchString(v.VideoInfo.SubTitle) {
			// 命中黑名单 则代表搜索不到
			c.common.Logger.Info("title in blacklist", "subTitle", v.VideoInfo.SubTitle)
			continue
		}
		if !param.MatchYear(v.VideoInfo.Year) {
			continue
		}

		var mediaType danmaku.MediaType
		if ssId < 0 {
			// 去掉标点之后 直接比较
			plainTitle := danmaku.MarkRegex.ReplaceAllLiteralString(v.VideoInfo.Title, "")
			plainKeyword := danmaku.MarkRegex.ReplaceAllLiteralString(keyword, "")
			if plainTitle != plainKeyword {
				continue
			}
			mediaType = danmaku.Movie
		} else {
			// 匹配标题 搜出来即是命中
			checkFirstSeason := false
			if ssId == 1 && original != v.VideoInfo.Title {
				checkFirstSeason = true
			}
			if ssId > 1 || checkFirstSeason {
				clearTitle := strings.ReplaceAll(v.VideoInfo.Title, " ", "")
				match := danmaku.SeasonTitleMatch.FindStringSubmatch(clearTitle)
				// 匹配到 第5季
				if len(match) > 1 {
					id, _ := strconv.ParseInt(match[1], 10, 64)
					clearTitle = danmaku.SeasonTitleMatch.ReplaceAllString(clearTitle, "第"+danmaku.ChineseNumberSlice[id-1]+"季")
					if clearTitle != keyword {
						continue
					}
				} else {
					// 没匹配到 也可能是中文 第五季
					if clearTitle != keyword {
						continue
					}
				}
			}
			if v.VideoInfo.SubjectDoc.VideoNum <= 0 {
				// 没有集数信息
				continue
			}
			mediaType = danmaku.Series
		}

		seriesItems, e := c.series(v.Doc.Id)
		if e != nil {
			c.common.Logger.Error(e.Error())
			continue
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

			// 如果是电影则再次比对title 有些电影是没匹配上，但是剧集里会有一些预告甚至垃圾视频
			if ssId < 0 && ep.ItemParams.Title != keyword {
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

		result = append(result, media)
	}

	return result, nil
}

var tencentExcludeRegex = regexp.MustCompile(`(全网搜|外站)`)

func (c *client) GetDanmaku(id string) ([]*danmaku.StandardDanmaku, error) {
	return c.getDanmakuByVid(id)
}

func (c *client) SearcherType() danmaku.Platform {
	return danmaku.Tencent
}
