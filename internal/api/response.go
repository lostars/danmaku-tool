package api

import (
	"danmu-tool/internal/utils"
	"encoding/json"
	"net/http"
)

var respLogger = utils.GetComponentLogger("response")

func ResponseJSON(w http.ResponseWriter, status int, result interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(result)
	if err != nil {
		respLogger.Error(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{})
	}
}
