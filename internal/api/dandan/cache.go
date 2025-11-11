package dandan

import (
	"bytes"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

var cache *ristretto.Cache[string, []byte]

func init() {
	danmaku.RegisterInitializer(&DanmakuCache{})
}

type DanmakuCache struct{}

func (d *DanmakuCache) ServerInit() error {
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

const dandanApiCacheC = "dandan_api_cache"

func CacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var cacheKey = ""
		if strings.Contains(r.URL.Path, "/comment/") {
			// episodeId
			cacheKey = path.Base(r.URL.Path)
			if cachedData, found := cache.Get(cacheKey); found {
				_, _ = w.Write(cachedData)
				utils.DebugLog(dandanApiCacheC, "cache loaded", "cacheKey", cacheKey)
				return
			}
		}

		rr := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rr, r)

		if cacheKey != "" && rr.statusCode == http.StatusOK {
			cacheData := rr.body.Bytes()
			success := cache.SetWithTTL(cacheKey, cacheData, int64(len(cacheData)), time.Second*3600) // 1h to expire
			if !success {
				utils.DebugLog(dandanApiCacheC, "cache set failed", "cacheKey", cacheKey)
			}
		}
	})
}

type responseRecorder struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.body == nil {
		r.body = new(bytes.Buffer)
	}
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}
