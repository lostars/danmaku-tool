package dandan

import (
	"danmaku-tool/internal/api"
	"danmaku-tool/internal/danmaku"
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

	// TODO 这些参数暂时不处理
	query := r.URL.Query()
	query.Get("from")        // int64
	query.Get("withRelated") // bool
	query.Get("chConvert")   // bool

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
		Id: numId,
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

	var param danmaku.MatchParam
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
