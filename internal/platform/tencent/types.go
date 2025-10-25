package tencent

type SeriesTab struct {
	Begin       int    `json:"begin"`
	End         int    `json:"end"`
	Selected    bool   `json:"selected"`
	PageContext string `json:"page_context"` // 用于剧集tab 获取ep的重要参数
	PageNum     string `json:"page_num"`
	PageSize    string `json:"page_size"`
}

const SeriesEPPageId = "vsite_episode_list"
const SeriesInfoPageId = "detail_page_introduction"

type SeriesItem struct {
	ItemId     string `json:"item_id"` // 等于vid？
	ItemType   string `json:"item_type"`
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
