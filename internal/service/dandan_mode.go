package service

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"sync"
)

func init() {
	realTimeMode := &realTimeData{
		platform: danmaku.GetPlatforms(),
		season:   make([]string, 0),
		episode:  make([]string, 0),
		lock:     sync.Mutex{},
	}
	sourceModes = map[string]DandanSourceMode{string(realTimeMode.Mode()): realTimeMode}
}

var sourceModes map[string]DandanSourceMode

func GetDandanSourceMode() DandanSourceMode {
	return sourceModes[config.GetConfig().DandanMode]
}

// DandanSourceMode dandan api 数据源接口
type DandanSourceMode interface {
	Match(param MatchParam) (*MatchResult, error)
	GetDanmaku(param CommentParam) (*CommentResult, error)
	Mode() Mode
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

type MatchParam struct {
	FileName      string `json:"fileName"`
	FileSize      int64  `json:"fileSize"`
	MatchMod      string `json:"matchMod"` // fileNameOnly
	VideoDuration int64  `json:"videoDuration"`
	FileHash      string `json:"fileHash"`
}

type MatchResult struct {
	Success      bool    `json:"success"`
	ErrorCode    int     `json:"errorCode"`
	ErrorMessage string  `json:"errorMessage"`
	IsMatched    bool    `json:"isMatched"`
	Matches      []Match `json:"matches"`
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
