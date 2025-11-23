package service

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/utils"
	"fmt"
	"time"

	"github.com/longbridgeapp/opencc"
)

var sourceModes = make(map[string]DandanSourceMode)

func RegisterSource(src DandanSourceMode) {
	sourceModes[string(src.Mode())] = src
}

func GetDandanSourceMode() DandanSourceMode {
	return sourceModes[config.GetDandan().Mode]
}

// DandanSourceMode dandan api 数据源接口
type DandanSourceMode interface {
	Match(param MatchParam) (*DanDanResult, error)
	SearchAnime(title string) *DanDanAnimeResult
	AnimeInfo(id string) (*DanDanAnimeInfoResult, error)
	GetDanmaku(param CommentParam) (*CommentResult, error)
	Mode() Mode
}

type Mode string

const (
	realTime = "real_time"
	file     = "file"
)

const dandanService = "dandan_service"

func (c CommentResult) Convert(convert int64) {
	var cc *opencc.OpenCC
	switch convert {
	case 0:
		return
	case 1:
		cc, _ = opencc.New("t2s")
	case 2:
		cc, _ = opencc.New("s2t")
	}
	if cc == nil {
		return
	}
	start := time.Now()
	for _, comment := range c.Comments {
		if text, e := cc.Convert(comment.M); e == nil {
			comment.M = text
		} else {
			utils.ErrorLog(dandanService, fmt.Sprintf("comment convert error: %s", e.Error()))
		}
	}
	utils.DebugLog(dandanService, "comment convert done", "cost_ms", time.Since(start).Milliseconds())
}

type CommentParam struct {
	From        int64 `form:"from"`
	WithRelated bool  `form:"withRelated"`
	Convert     int64 `form:"chConvert"`
	Id          int64
}

type MatchParam struct {
	FileName        string `json:"fileName"`
	FileSize        int64  `json:"fileSize"`
	MatchMod        string `json:"matchMod"` // fileNameOnly
	DurationSeconds int64  `json:"videoDuration"`
	FileHash        string `json:"fileHash"`
}

type DanDanResult struct {
	DanDanResultInfo
	// match result
	IsMatched bool    `json:"isMatched"`
	Matches   []Match `json:"matches"`
}

type DanDanResultInfo struct {
	Success      bool   `json:"success"`
	ErrorCode    int    `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

type DanDanAnimeResult struct {
	DanDanResultInfo
	Anime []AnimeResult `json:"animes"`
}

type DanDanAnimeInfoResult struct {
	DanDanResultInfo
	Bangumi *AnimeResult `json:"bangumi"`
}

type AnimeResult struct {
	AnimeId      int64           `json:"animeId"`
	BangumiId    string          `json:"bangumiId"`
	AnimeTitle   string          `json:"animeTitle"`
	Type         string          `json:"type"`
	TypeDesc     string          `json:"typeDescription"`
	ImageUrl     string          `json:"imageUrl"`
	StartDate    string          `json:"startDate"` // 2025-10-31T02:45:58.049Z
	EpisodeCount int             `json:"episodeCount"`
	Rating       int             `json:"rating"`
	Favorite     bool            `json:"isFavorited"`
	Episodes     []EpisodeResult `json:"episodes,omitempty"`
}

type EpisodeResult struct {
	SeasonId      string `json:"seasonId"`
	EpisodeId     int64  `json:"episodeId"`
	EpisodeTitle  string `json:"episodeTitle"`
	EpisodeNumber string `json:"episodeNumber"`
}

type Match struct {
	EpisodeId    int64  `json:"episodeId"` // 关键信息在于这个id，用于后续获取弹幕
	AnimeId      int    `json:"animeId"`
	AnimeTitle   string `json:"animeTitle"`
	EpisodeTitle string `json:"episodeTitle"`    // 第1话 天界的咲稻姬
	Type         string `json:"type"`            // tvseries
	TypeDesc     string `json:"typeDescription"` // TV动画
	Shift        int    `json:"shift"`
}

type CommentResult struct {
	Count    int64      `json:"count"`
	Comments []*Comment `json:"comments"`
}

type Comment struct {
	CID int64  `json:"cid"`
	P   string `json:"p"`
	M   string `json:"m"`
}
