package service

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
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
	Match(param MatchParam) (*DanDanResult, error)
	SearchAnime(title string) *DanDanResult
	AnimeInfo(id string) (*DanDanResult, error)
	GetDanmaku(param CommentParam) (*CommentResult, error)
	Mode() Mode
}

type Mode string

const (
	realTime = "real_time"
)

type CommentParam struct {
	From        int64
	WithRelated bool
	Convert     bool
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
	Success      bool   `json:"success"`
	ErrorCode    int    `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
	// match result
	IsMatched bool    `json:"isMatched"`
	Matches   []Match `json:"matches"`
	// search result
	Anime   []AnimeResult `json:"animes"`
	Bangumi *AnimeResult  `json:"bangumi"`
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
	Episodes     []EpisodeResult `json:"episodes"`
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
