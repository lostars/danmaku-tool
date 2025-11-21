package dandan

import (
	"bytes"
	"crypto/md5"
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
	"danmaku-tool/internal/web"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

var cache *ristretto.Cache[string, []byte]

func init() {
	cacheInterface := &DanmakuCache{}
	danmaku.RegisterFinalizer(cacheInterface)
	danmaku.Register(cacheInterface)
}

type DanmakuCache struct{}

func (d *DanmakuCache) ServerInit() error {
	if config.GetDandan().CacheTimeout <= 0 {
		utils.InfoLog(dandanApiCacheC, "ristretto cache disabled")
		return nil
	}
	c, err := ristretto.NewCache(&ristretto.Config[string, []byte]{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 29, // maximum cost of cache 512M
		BufferItems: 64,      // number of keys per Get buffer.
	})
	if err != nil {
		return err
	}
	cache = c
	return nil
}

func (d *DanmakuCache) Finalize() error {
	if cache != nil {
		cache.Close()
	}
	return nil
}

const dandanApiCacheC = "dandan_api_cache"

func CacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cache == nil {
			rr := &responseRecorder{StatusRecorder: &web.StatusRecorder{ResponseWriter: w}}
			next.ServeHTTP(rr, r)
			return
		}

		var key = cacheKey(r)
		if cachedData, found := cache.Get(key); found {
			if _, err := w.Write(cachedData); err != nil {
				utils.ErrorLog(dandanApiCacheC, fmt.Sprintf("write cache error: %s", err.Error()))
			}
			utils.DebugLog(dandanApiCacheC, "cache loaded", "cacheKey", key)
			return
		}

		rr := &responseRecorder{StatusRecorder: &web.StatusRecorder{ResponseWriter: w}}
		next.ServeHTTP(rr, r)

		if rr.Status == http.StatusOK {
			cacheData := rr.body.Bytes()
			cacheDuration := time.Duration(config.GetDandan().CacheTimeout * 1e9)
			if success := cache.SetWithTTL(key, cacheData, int64(len(cacheData)), cacheDuration); !success {
				utils.ErrorLog(dandanApiCacheC, "cache set failed", "cacheKey", key)
			}
		}
	})
}

func cacheKey(r *http.Request) string {
	bodyBytes, _ := io.ReadAll(r.Body)
	// save body for next handler
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var body map[string]interface{}
	_ = json.Unmarshal(bodyBytes, &body)
	data := map[string]interface{}{
		"path":  r.URL.Path,
		"query": r.URL.Query(),
		"body":  body,
	}
	b, _ := json.Marshal(data)
	return fmt.Sprintf("%x", md5.Sum(b))
}

type responseRecorder struct {
	*web.StatusRecorder
	body *bytes.Buffer
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.body == nil {
		r.body = new(bytes.Buffer)
	}
	r.body.Write(b)
	return r.StatusRecorder.Write(b)
}
