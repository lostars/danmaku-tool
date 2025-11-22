package dandan

import (
	"danmaku-tool/internal/service"
	"danmaku-tool/internal/utils"
	"danmaku-tool/internal/web"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

const dandanApiC = "dandan_api"

func CommentHandler(w http.ResponseWriter, r *http.Request) {

	id := chi.URLParam(r, "id")

	numId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		web.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
		return
	}

	query := r.URL.Query()
	var from int64
	if query.Get("from") != "" {
		from, err = strconv.ParseInt(query.Get("from"), 10, 64)
		if err != nil {
			web.ResponseJSON(w, http.StatusBadRequest, map[string]string{
				"message": "invalid from parameter",
			})
			return
		}
	}
	convert, _ := strconv.ParseBool(query.Get("chConvert"))
	withRelated, _ := strconv.ParseBool(query.Get("withRelated"))

	comment, err := source.GetDanmaku(service.CommentParam{
		Id:          numId,
		Convert:     convert,
		WithRelated: withRelated,
		From:        from,
	})
	if err != nil {
		web.ResponseJSON(w, http.StatusBadRequest, map[string]string{
			"message": err.Error(),
		})
		return
	}
	code := http.StatusOK
	if len(comment.Comments) < 1 {
		code = http.StatusNotFound
	}

	web.ResponseJSON(w, code, comment)
}

func MatchHandler(w http.ResponseWriter, r *http.Request) {

	var param service.MatchParam
	err := web.DecodeJSONBody(w, r, &param)
	if err != nil {
		web.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
		return
	}

	result, err := source.Match(param)
	utils.DebugLog(dandanApiC, fmt.Sprintf("request original param: %v", param))
	if err != nil {
		web.ResponseJSON(w, http.StatusBadRequest, map[string]string{
			"message": err.Error(),
		})
		return
	}
	code := http.StatusOK
	if len(result.Matches) < 1 {
		code = http.StatusNotFound
	}

	web.ResponseJSON(w, code, result)
}

func SearchAnime(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	keyword := query.Get("keyword")
	query.Get("type")

	result := source.SearchAnime(keyword)
	code := http.StatusOK
	if len(result.Anime) < 1 {
		code = http.StatusNotFound
	}
	web.ResponseJSON(w, code, result)
}

func AnimeInfo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	result, err := source.AnimeInfo(id)
	if err != nil {
		web.ResponseJSON(w, http.StatusBadRequest, map[string]string{
			"message": err.Error(),
		})
		return
	}
	code := http.StatusOK
	if result.Bangumi == nil {
		code = http.StatusNotFound
	}
	web.ResponseJSON(w, code, result)
}
