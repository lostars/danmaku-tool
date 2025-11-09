package tencent

import "regexp"

var tencentExcludeRegex = regexp.MustCompile(`(全网搜|外站)`)

const SeriesEPPageId = "vsite_episode_list"
const SeriesInfoPageId = "detail_page_introduction"

type SeriesTab struct {
	Begin       int    `json:"begin"`
	End         int    `json:"end"`
	Selected    bool   `json:"selected"`
	PageContext string `json:"page_context"` // 用于剧集tab 获取ep的重要参数
	PageNum     string `json:"page_num"`
	PageSize    string `json:"page_size"`
}

type SeriesItem struct {
	ItemId     string `json:"item_id"`
	ItemType   string `json:"item_type"` // =28 一部电影的 多集？
	ItemParams struct {
		// 以下是 page_id=vsite_episode_list 返回的剧集ep信息
		VID          string `json:"vid"`
		Duration     string `json:"duration"`       // 时长：秒
		CTitleOutput string `json:"c_title_output"` // 01
		// 该字段在 page_id=detail_page_introduction 返回剧集名称
		// page_id=vsite_episode_list 返回ep集数
		Title     string `json:"title"`      // 1
		IsTrailer string `json:"is_trailer"` // 1=预告 0=否
		CID       string `json:"cid"`

		// 以下是 page_id=detail_page_introduction 返回的剧集信息
		ReportCID string `json:"report.cid"`
		// 2=剧集 1=电影 10=综艺 3=动漫 9=纪录片 4=体育
		Type string `json:"type"`
		// 剧集集数
		EpisodeAll string `json:"episode_all"`
		// 1=已完结？需要确认
		AnimeUpdateStatusId string `json:"anime_update_status_id"`
	} `json:"item_params"`
}

type SeriesReqParam struct {
	HasCache   int                `json:"has_cache"`
	PageParams SeriesReqPageParam `json:"page_params"`
}
type SeriesReqPageParam struct {
	ReqFrom string `json:"req_from"`
	// detail_page_introduction 获取剧集信息
	// vsite_episode_list 剧集ep信息
	PageId         string `json:"page_id"`
	PageType       string `json:"page_type"`
	IdType         string `json:"id_type"`
	PageSize       string `json:"page_size"`
	CID            string `json:"cid"` // 剧集id
	VID            string `json:"vid"` // 视频id
	LID            string `json:"lid"`
	PageNum        string `json:"page_num"`
	PageContext    string `json:"page_context"` // 这是个json字符串，页面上剧集列表里的tab信息，1-30集 31-50集
	DetailPageType string `json:"detail_page_type"`
}

type SeriesResult struct {
	Ret  int    `json:"ret"`
	Msg  string `json:"msg"`
	Data struct {
		ModuleListData []struct {
			ModuleData []struct {
				ModuleParams struct {
					Tabs string `json:"tabs"` // 这是个json字符串，页面上剧集列表里的tab信息，1-30集 31-50集
				} `json:"module_params"`
				ItemDataLists struct {
					ItemData []SeriesItem `json:"item_datas"`
				} `json:"item_data_lists"`
			} `json:"module_datas"`
		} `json:"module_list_datas"`
	} `json:"data"`
}

type DanmakuResult struct {
	BarrageList []struct {
		Content    string `json:"content"`
		Id         string `json:"id"`
		UpCount    string `json:"up_count"`    // 点赞数？
		CreateTime string `json:"create_time"` // 1715077975
		// {\"color\":\"ffffff\",\"gradient_colors\":[\"44EB1F\",\"44EB1F\"],\"position\":1}
		// 颜色信息 json
		ContentStyle string `json:"content_style"`
		TimeOffset   string `json:"time_offset"` // 弹幕偏移时间 ms
	} `json:"barrage_list"`
}

type DanmakuColorResult struct {
	Color          string   `json:"color"`
	GradientColors []string `json:"gradient_colors"`
	Position       int      `json:"position"`
}

type DanmakuSegmentResult struct {
	Ret  int    `json:"ret"`
	Msg  string `json:"msg"`
	Data struct {
		SegmentSpan  string `json:"segment_span"`
		SegmentStart string `json:"segment_start"`
		SegmentIndex map[string]struct {
			SegmentStart string `json:"segment_start"`
			SegmentName  string `json:"segment_name"`
		} `json:"segment_index"`
	} `json:"data"`
}

type SearchResult struct {
	Ret  int    `json:"ret"`
	Msg  string `json:"msg"`
	Data struct {
		NormalList struct {
			ItemList []SearchResultItem `json:"itemList"`
		} `json:"normalList"`
		AreaBoxList []struct {
			BoxId    string             `json:"boxId"`
			ItemList []SearchResultItem `json:"itemList"`
		} `json:"areaBoxList"`
	} `json:"data"`
}

type SearchResultItem struct {
	Doc struct {
		Id string `json:"id"` // civ 重要数据 用于获取vid
	} `json:"doc"`
	VideoInfo struct {
		VideoType int    `json:"videoType"` // 上面定义的Type
		Desc      string `json:"descrip"`
		ImgUrl    string `json:"imgUrl"`
		TypeName  string `json:"typeName"` // 电视剧/电影
		// 年份
		Year       int    `json:"year"`
		Title      string `json:"title"`    // 标题 可能包含第几季
		SubTitle   string `json:"subTitle"` // 包含 全网搜 关键字则代表匹配失败
		Status     int    `json:"status"`   // 可能是完结或者已开播的意思？ 1=未开播 0=正常
		SubjectDoc struct {
			VideoNum int `json:"videoNum"` // 剧集集数
		} `json:"subjectDoc"`
	} `json:"videoInfo"`
}

type SearchParam struct {
	Version    string `json:"version"`    // 25101301
	ClientType int    `json:"clientType"` // 1
	// 非常重要的字段，少了会报错 ret = 400/500 哪怕是空字符串
	// 该字段用于传递 网页搜索结果页面上 对结果进行筛选的tab的信息，比如全部 /动画电影 /动画剧集
	FilterValue string `json:"filterValue"`
	Query       string `json:"query"`      // 搜索关键字
	PageNum     int    `json:"pagenum"`    // 0
	IsPrefetch  bool   `json:"isPrefetch"` // true
	PageSize    int    `json:"pagesize"`   // 30
	QueryFrom   int    `json:"queryFrom"`  // 102

	// 下面的字段用于调试，保不住哪天少了个字段接口又会报错

	UUID          string          `json:"uuid"`
	Retry         int             `json:"retry"`
	SearchDataKey string          `json:"searchDatakey"`
	TransInfo     string          `json:"transInfo"`
	NeedQc        bool            `json:"isneedQc"` // true
	PreQid        string          `json:"preQid"`
	AdClientInfo  string          `json:"adClientInfo"`
	ExtraInfo     SearchExtraInfo `json:"extraInfo"`
}

type SearchExtraInfo struct {
	IsNewMarkLabel  string `json:"isNewMarkLabel"`
	MultiTerminalPc string `json:"multi_terminal_pc"`
	ThemeType       string `json:"themeType"`
	// 该字段应该是一个 json 字符串，如果错误的传递成 {} 或者其他非法json，可能导致接口无法搜索出数据
	SugRelatedIds string `json:"sugRelatedIds"`
	AppVersion    string `json:"appVersion"`
}
