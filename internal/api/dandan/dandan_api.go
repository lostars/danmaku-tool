package dandan

import (
	"danmaku-tool/internal/api"
	"danmaku-tool/internal/service"
	"danmaku-tool/internal/utils"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func CommentHandler(w http.ResponseWriter, r *http.Request) {

	token := chi.URLParam(r, "token")
	id := chi.URLParam(r, "id")

	numId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
		return
	}

	query := r.URL.Query()
	var from int64
	if query.Get("from") != "" {
		from, err = strconv.ParseInt(query.Get("from"), 10, 64)
		if err != nil {
			api.ResponseJSON(w, http.StatusBadRequest, map[string]string{
				"message": "invalid from parameter",
			})
			return
		}
	}
	convert, _ := strconv.ParseBool(query.Get("chConvert"))
	withRelated, _ := strconv.ParseBool(query.Get("withRelated"))

	dandanLogger := utils.GetComponentLogger("dandan-api")
	dandanLogger.Info("comment api requested", "token", token, "id", id)

	mode := service.GetDandanSourceMode()
	if mode == nil {
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{
			"message": "no available source",
		})
		return
	}
	comment, err := mode.GetDanmaku(service.CommentParam{
		Id:          numId,
		Convert:     convert,
		WithRelated: withRelated,
		From:        from,
	})
	if err != nil {
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{
			"message": err.Error(),
		})
		return
	}

	api.ResponseJSON(w, http.StatusOK, comment)
}

func MatchHandler(w http.ResponseWriter, r *http.Request) {

	var param service.MatchParam
	err := api.DecodeJSONBody(w, r, &param)
	if err != nil {
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{})
		return
	}

	mode := service.GetDandanSourceMode()
	if mode == nil {
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{
			"message": "no available source",
		})
		return
	}
	result, err := mode.Match(param)
	utils.GetComponentLogger("dandan-api").Debug(fmt.Sprintf("request original param: %v", param))
	if err != nil {
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{
			"message": err.Error(),
		})
		return
	}

	api.ResponseJSON(w, http.StatusOK, result)
}

func SearchAnime(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	keyword := query.Get("keyword")
	query.Get("type")

	mode := service.GetDandanSourceMode()
	if mode == nil {
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{
			"message": "no available source",
		})
		return
	}
	result := mode.SearchAnime(keyword)

	api.ResponseJSON(w, http.StatusOK, result)
}

func AnimeInfo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mode := service.GetDandanSourceMode()
	if mode == nil {
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{
			"message": "no available source",
		})
		return
	}
	result, err := mode.AnimeInfo(id)
	if err != nil {
		api.ResponseJSON(w, http.StatusBadRequest, map[string]string{
			"message": err.Error(),
		})
		return
	}
	api.ResponseJSON(w, http.StatusOK, result)
}
