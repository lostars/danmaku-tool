package service

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"sync"
)

func init() {
	cacheMapper := &realTimeData{
		lock: sync.RWMutex{},
	}
	danmaku.RegisterInitializer(cacheMapper)
	sourceModes = map[string]DandanSourceMode{string(cacheMapper.Mode()): cacheMapper}
}

var sourceModes map[string]DandanSourceMode

func GetDandanSourceMode() DandanSourceMode {
	return sourceModes[config.GetConfig().DandanMode]
}

// DandanSourceMode dandan api 数据源接口
type DandanSourceMode interface {
	Match(param danmaku.MatchParam) (*DanDanResult, error)
	GetDanmaku(param CommentParam) (*CommentResult, error)
	Mode() Mode
}

type SourceRelease interface {
	ReleaseSource() error
}

type Mode string

const (
	realTime = "real_time"
	database = "database"
)

type CommentParam struct {
	From        int64
	WithRelated bool
	Convert     bool
	Id          int64
}

type DanDanResult struct {
	Success      bool   `json:"success"`
	ErrorCode    int    `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
	// match result
	IsMatched bool    `json:"isMatched"`
	Matches   []Match `json:"matches"`
	// search result
	Anime []AnimeResult `json:"animes"`
}

type AnimeResult struct {
	AnimeId      int64  `json:"animeId"`
	BangumiId    string `json:"bangumiId"`
	AnimeTitle   string `json:"animeTitle"`
	Type         string `json:"type"`
	TypeDesc     string `json:"typeDescription"`
	ImageUrl     string `json:"imageUrl"`
	StartDate    string `json:"startDate"` // 2025-10-31T02:45:58.049Z
	EpisodeCount int    `json:"episodeCount"`
	Rating       int    `json:"rating"`
	Favorite     bool   `json:"isFavorited"`
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
