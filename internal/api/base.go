package api

import (
	"danmaku-tool/internal/utils"
	"encoding/json"
	"fmt"
	"net/http"
)

func ResponseJSON(w http.ResponseWriter, status int, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if result == nil {
		result = map[string]interface{}{"status": status}
	}
	err := json.NewEncoder(w).Encode(result)
	if err != nil {
		http.Error(w, fmt.Sprintf("encode json error: %s", err), http.StatusInternalServerError)
		utils.GetComponentLogger("base").Error(err.Error())
	}
}

func DecodeJSONBody(w http.ResponseWriter, r *http.Request, target interface{}) error {
	defer utils.SafeClose(r.Body)
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
		return fmt.Errorf("content type must be application/json")
	}

	err := json.NewDecoder(r.Body).Decode(target)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid request payload: %s", err), http.StatusBadRequest)
		return fmt.Errorf("json decode error: %w", err)
	}

	return nil
}
