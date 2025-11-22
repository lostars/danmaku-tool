package web

import (
	"danmaku-tool/internal/utils"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

type ErrBadRequest struct {
	Message string
}

func (e ErrBadRequest) Error() string {
	return e.Message
}

func ErrBadReqeustOf(format string, a ...any) error {
	return ErrBadRequest{fmt.Sprintf(format, a...)}
}

func IsBadRequest(err error) bool {
	var errBadRequest ErrBadRequest
	return errors.As(err, &errBadRequest)
}

type StatusRecorder struct {
	http.ResponseWriter
	WroteHeader bool
	Status      int
}

func (r *StatusRecorder) WriteHeader(status int) {
	if !r.WroteHeader {
		r.WroteHeader = true
		r.ResponseWriter.WriteHeader(status)
	}
	r.Status = status
}

func (r *StatusRecorder) Write(b []byte) (int, error) {
	if !r.WroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}

func GetRealIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func ErrResponseJSON(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if IsBadRequest(err) {
		status = http.StatusBadRequest
	}
	result := map[string]interface{}{"status": status, "message": err.Error()}
	ResponseJSON(w, status, result)
}

func ResponseJSON(w http.ResponseWriter, status int, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err == nil {
		w.WriteHeader(status)
	} else {
		http.Error(w, fmt.Sprintf("encode json error: %s", err), http.StatusInternalServerError)
		utils.ErrorLog("base", err.Error())
	}
}

func DecodeJSONBody(r *http.Request, target any) error {
	defer utils.SafeClose(r.Body)
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("content type must be application/json")
	}

	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		return fmt.Errorf("json decode error: %w", err)
	}

	return nil
}
