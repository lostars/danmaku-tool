package youku

import (
	"regexp"
	"strings"
)

var videoRegex = regexp.MustCompile(`<script>window\.__INITIAL_DATA__\s=(\{.*});</script>`)
var pageRegex = regexp.MustCompile(`<script>window\.__PAGE_CONF__\s=(\{.*});`)
var matchVIDRegex = regexp.MustCompile(`/v_show/id_([a-zA-Z0-9=]+)\.html`)

var blacklistRegex = regexp.MustCompile(`短剧`)
var blacklistCatsRegex = regexp.MustCompile(`游戏|文化`)
var yearMatchRegex = regexp.MustCompile(`\s(\d{4})\s·`)
var blacklistTitleRegex = regexp.MustCompile(`幕后|赏析|解析|解读|剖析|揭秘|音频|观影指南`)

type APIResult struct {
	API  string `json:"api"`
	Data struct {
		// json字符串 DanmakuResult
		Result string `json:"result"`
		Nodes  []struct {
			// 单个视频只有第一个 剧集则有第二个
			Nodes []struct {
				Nodes []struct {
					Data NodeData `json:"data"`
					// 层级 用于调试
					Level int `json:"level"`
				} `json:"nodes"`
				Level int `json:"level"`
			} `json:"nodes"`
			Level int `json:"level"`
		} `json:"nodes"`
		Level int `json:"level"`
	} `json:"data"`
	TraceId string   `json:"traceId"`
	V       string   `json:"v"`
	Ret     []string `json:"ret"`
}

type NodeData struct {
	// show info
	IsYouku        int    `json:"isYouku"` // 是否优酷平台剧集
	HasYouku       int    `json:"hasYouku"`
	SourceName     string `json:"sourceName"` // 来源 优酷/腾讯 ...
	IsTrailer      int    `json:"isTrailer"`
	RealShowId     string `json:"realShowId"`     // showId
	EpisodeTotal   int    `json:"episodeTotal"`   // ep数量
	MediaCompleted int    `json:"mediaCompleted"` // 是否完结
	TempTitle      string `json:"tempTitle"`
	ThumbUrl       string `json:"thumbUrl"`
	Info           string `json:"info"`
	Cats           string `json:"cats"` // 分类

	PosterDTO struct {
		IconCorner struct {
			TagText string `json:"tagText"`
			TagType int    `json:"tagType"`
		} `json:"iconCorner"`
	} `json:"posterDTO"`

	FeatureDTO struct {
		// 电视剧 · 2009 · 中国
		Text string `json:"text"`
	} `json:"featureDTO"`

	// ep info
	ShowVideoStage string `json:"showVideoStage"` // 1 第几集
	Seconds        string `json:"seconds"`        // 时长 秒
	VideoId        string `json:"videoId"`        // 视频字符串id
	VID            string `json:"vid"`            // 视频数字id
	Title          string `json:"title"`          // "仙剑奇侠传三 01"
	OrderId        int    `json:"orderId"`        // 排序id
}

func (a *APIResult) success() bool {
	for _, s := range a.Ret {
		if strings.HasPrefix(s, "SUCCESS") {
			return true
		}
	}
	return false
}

type DanmakuResult struct {
	Code int    `json:"code"`
	Cost string `json:"cost"`
	Data struct {
		Result []struct {
			Content string `json:"content"` // 内容
			Mat     int    `json:"mat"`     // 所在弹幕分片
			PlayAt  int64  `json:"playat"`  // 弹幕毫秒数
			// DanmakuPropertyResult 位置 颜色 大小信息
			Property string `json:"propertis"`
			Status   int    `json:"status"`
			Type     int    `json:"type"`
			UID      string `json:"uid"`
			UID2     int64  `json:"uid2"`
			Ver      int    `json:"ver"`
		} `json:"result"`
	} `json:"data"`
}

type DanmakuPropertyResult struct {
	Size       int `json:"size"`
	Alpha      int `json:"alpha"`
	Pos        int `json:"pos"`
	MarkSource int `json:"markSource"`
	Color      int `json:"color"` // int颜色
}

type VideoInfoFromHtml struct {
	Title          string `json:"title"` // "仙剑奇侠传 第一部 01"
	IsShow         bool   `json:"isShow"`
	ShowId         string `json:"showId"`
	ShowName       string `json:"showname"`       // "仙剑奇侠传 第一部"
	Seconds        string `json:"seconds"`        // 秒数字符串
	Unit           string `json:"unit"`           // 集
	StageStr       string `json:"stageStr"`       // 第1集
	ShowVideoStage string `json:"showVideostage"` // 1
	Completed      int    `json:"completed"`      // 是否完结
}

type ShowInfoFromHtml struct {
	PageMap struct {
		Extra struct {
			ShowCategory string `json:"showCategory"`
			ShowImgV     string `json:"showImgV"`
		}
	} `json:"pageMap"`
	ModuleList []struct {
		// 10001 NORMAL
		Type       int    `json:"type"`
		TypeName   string `json:"typeName"`
		Components []struct {
			// 10013 Web播放页选集组件 此时为剧集列表信息
			Type     int    `json:"type"`
			TypeName string `json:"typeName"`
			ItemList []struct {
				Id          int64  `json:"id"`           // vid 数字id
				ActionValue string `json:"action_value"` // vid 字符串
				Title       string `json:"title"`        // 司藤 第1集
				StageIndex  int    `json:"stageIndex"`   // 2 编号
			} `json:"itemList"`
		} `json:"components"`
	} `json:"moduleList"`
}
