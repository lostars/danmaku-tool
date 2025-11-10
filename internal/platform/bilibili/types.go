package bilibili

import "danmaku-tool/internal/danmaku"

type SeriesInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  struct {
		Cover string `json:"cover"`
		// 当前EP所在Season所有EPs 电影也会返回数据 只有一条
		Episodes []struct { // 0 第一集 1 第二集 预告可能也会在里面
			AId         int64  `json:"aid"`
			BVId        string `json:"bvid"`
			CId         int64  `json:"cid"`
			Duration    int64  `json:"duration"` // in Millisecond
			EPId        int64  `json:"ep_id"`
			SectionType int    `json:"section_type"` // 1 是预告之类的 0是正常剧集？？
			Link        string `json:"link"`
			Title       string `json:"title"`      // 1 集数编号
			ShowTitle   string `json:"show_title"` // 第1话 阿七的特别任务
			PubTime     int64  `json:"pub_time"`
			// 分辨率信息
			Dimension struct {
				Height int `json:"height"`
				Rotate int `json:"rotate"`
				Width  int `json:"width"`
			} `json:"dimension"`
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

func parseMediaType(mediaType int) danmaku.MediaType {
	switch mediaType {
	case 2:
		return danmaku.Movie
	}
	return danmaku.Series
}

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
			// 发布日期 in seconds
			PubTime int64  `json:"pubtime"`
			Desc    string `json:"desc"`
			EPSize  int    `json:"ep_size"`
			EPs     []struct {
				Id         int64  `json:"id"`
				Title      string `json:"title"`       // 第几集 13
				IndexTitle string `json:"index_title"` // 和 title 一样？
				LongTitle  string `json:"long_title"`  // 初入星海11
			} `json:"eps"` // 完整数据
		} `json:"result"`
	} `json:"data"`
}

func isSeries(mediaType int) bool {
	return mediaType != 2
}
