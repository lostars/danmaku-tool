package youku

import "strings"

type APIResult struct {
	API  string `json:"api"`
	Data struct {
		// json字符串 DanmakuResult
		Result string `json:"result"`
	} `json:"data"`
	TraceId string   `json:"traceId"`
	V       string   `json:"v"`
	Ret     []string `json:"ret"`
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
			Property string `json:"properties"`
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
	Pos        int `json:"pos"` // 3=顶部
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
