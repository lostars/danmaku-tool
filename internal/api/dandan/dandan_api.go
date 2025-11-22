package dandan

import (
	"danmaku-tool/internal/service"
	"danmaku-tool/internal/web"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func CommentHandler(w http.ResponseWriter, r *http.Request) {

	commentId, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		web.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
		return
	}

	query := r.URL.Query()
	from, err := strconv.ParseInt(query.Get("from"), 10, 64)
	if err != nil {
		web.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
		return
	}
	convert, err := strconv.ParseBool(query.Get("chConvert"))
	if err != nil {
		web.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
		return
	}
	withRelated, err := strconv.ParseBool(query.Get("withRelated"))
	if err != nil {
		web.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
		return
	}

	comment, err := source.GetDanmaku(service.CommentParam{
		Id:          commentId,
		Convert:     convert,
		WithRelated: withRelated,
		From:        from,
	})
	if err != nil {
		web.ErrResponseJSON(w, err)
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
	if err := web.DecodeJSONBody(r, &param); err != nil {
		web.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
		return
	}

	result, err := source.Match(param)
	if err != nil {
		web.ErrResponseJSON(w, err)
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
		web.ErrResponseJSON(w, err)
		return
	}
	code := http.StatusOK
	if result.Bangumi == nil {
		code = http.StatusNotFound
	}
	web.ResponseJSON(w, code, result)
}
