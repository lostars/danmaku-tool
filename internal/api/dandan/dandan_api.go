package dandan

import (
	"danmu-tool/internal/api"
	"danmu-tool/internal/danmaku"
	"danmu-tool/internal/utils"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type CommentResult struct {
	Count    int64      `json:"count"`
	Comments []*Comment `json:"comments"`
}

type Comment struct {
	CID int64  `json:"cid"`
	P   string `json:"p"`
	M   string `json:"m"`
}

func CommentHandler(w http.ResponseWriter, r *http.Request) {

	token := chi.URLParam(r, "token")
	id := chi.URLParam(r, "id")

	numId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
		return
	}
	id = danmaku.DecodeEpisodeId(numId)

	// TODO 这些参数暂时不处理
	query := r.URL.Query()
	query.Get("from")        // int64
	query.Get("withRelated") // bool
	query.Get("chConvert")   // bool

	dandanLogger := utils.GetComponentLogger("dandan-api")
	dandanLogger.Info("comment api requested", "token", token, "id", id)

	ids := strings.Split(id, "_")
	if len(ids) != 3 {
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
		return
	}
	var scraper = danmaku.ManagerOfDanmaku.Searchers[ids[0]]
	if scraper == nil {
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
		return
	}
	data, err := scraper.GetDanmaku(id)
	if err != nil {
		dandanLogger.Error("get danmaku error", err)
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{
			"message": err.Error(),
		})
		return
	}
	comment := CommentResult{
		Count:    int64(len(data)),
		Comments: make([]*Comment, 0, len(data)),
	}

	for _, d := range data {
		comment.Comments = append(comment.Comments, &Comment{
			CID: time.Now().Unix(),
			M:   d.Content,
			P:   d.GenDandanAttribute(),
		})
	}

	api.ResponseJSON(w, http.StatusOK, comment)
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

func MatchHandler(w http.ResponseWriter, r *http.Request) {

	var param MatchParam
	err := api.DecodeJSONBody(w, r, &param)
	if err != nil {
		return
	}
	//{
	// "fileName": "天穗之咲稻姬 S01E01",
	// "fileSize": 0,
	// "matchMode": "fileNameOnly",
	// "videoDuration": 0,
	// "fileHash": "123d05841b9456ccc7420b3f0bb21c3b"
	//}
	strs := strings.Split(param.FileName, " ")
	if strs[0] == "" {
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
		return
	}
	searchTitle := strs[0]
	searchMovies := false
	if len(strs) == 1 || strs[1] == "" {
		searchMovies = true
	}
	var epId int64
	dandanLogger := utils.GetComponentLogger("dandan-api")
	if len(strs) > 1 && strs[1] != "" {
		sStrs := strings.Split(strs[1], "E")
		if len(sStrs) <= 1 {
			dandanLogger.Error("search series but parse ep id error", "title", searchTitle)
			api.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
			return
		}
		epId, err = strconv.ParseInt(sStrs[1], 10, 64)
		if err != nil {
			dandanLogger.Error("search series but parse ep id error", "title", searchTitle)
			api.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
			return
		}
	}

	var result = MatchResult{
		Matches: make([]Match, 0, 10),
		Success: true,
	}

	for _, s := range danmaku.ManagerOfDanmaku.Searchers {
		media, err := s.Search(searchTitle)
		if err != nil {
			dandanLogger.Error(err.Error(), "searchType", s.SearcherType(), "title", searchTitle)
			continue
		}
		dandanLogger.Debug("search success", "searchType", s.SearcherType(), "title", searchTitle)
		for _, m := range media {
			if m.Episodes == nil || len(m.Episodes) == 0 {
				continue
			}
			if searchMovies {
				result.IsMatched = true
				result.Matches = append(result.Matches, Match{
					EpisodeId:    danmaku.GenEpisodeId(s.SearcherType(), m.Id, m.Episodes[0].Id),
					AnimeTitle:   m.Title,
					EpisodeTitle: m.Episodes[0].Title,
				})
			} else {
				for _, ep := range m.Episodes {
					epStr := strconv.FormatInt(epId, 10)
					if ep.EpisodeId == epStr {
						dandanLogger.Info("ep match success", "searchType", s.SearcherType(), "title", searchTitle, "ep", ep.EpisodeId)
						result.IsMatched = true
						result.Matches = append(result.Matches, Match{
							EpisodeId:    danmaku.GenEpisodeId(s.SearcherType(), m.Id, ep.Id),
							AnimeTitle:   m.Title,
							EpisodeTitle: ep.EpisodeId,
						})
						break
					}
				}
			}
		}
	}

	api.ResponseJSON(w, http.StatusOK, result)
}
