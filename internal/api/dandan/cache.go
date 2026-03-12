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
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"golang.org/x/sync/singleflight"
)

var cache *DanmakuCache

func init() {
	cache = &DanmakuCache{}
	danmaku.RegisterFinalizer(cache)
	danmaku.Register(cache)
}

type DanmakuCache struct {
	cache         *ristretto.Cache[string, []byte]
	g             singleflight.Group
	waitingCounts sync.Map
	maxWaiters    int32
}

func (d *DanmakuCache) Priority() int {
	return 10
}

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
	cache.cache = c
	cache.maxWaiters = 10
	return nil
}

func (d *DanmakuCache) Finalize() error {
	if cache.cache != nil {
		cache.cache.Close()
	}
	return nil
}

const dandanApiCacheC = "dandan_api_cache"

func CacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cache.cache == nil {
			rr := &responseRecorder{StatusRecorder: &web.StatusRecorder{ResponseWriter: w}}
			next.ServeHTTP(rr, r)
			return
		}

		var key = cacheKey(r)
		if cachedData, found := cache.cache.Get(key); found {
			w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", config.GetDandan().CacheTimeout))
			if _, err := w.Write(cachedData); err != nil {
				utils.ErrorLog(dandanApiCacheC, fmt.Sprintf("write cache error: %s", err.Error()))
			}
			utils.DebugLog(dandanApiCacheC, "cache loaded", "cacheKey", key)
			return
		}

		// 检查最大实际并发
		actual, _ := cache.waitingCounts.LoadOrStore(key, new(int32))
		counter := actual.(*int32)
		currentWaiting := atomic.AddInt32(counter, 1)
		defer atomic.AddInt32(counter, -1)
		if currentWaiting > cache.maxWaiters {
			utils.WarnLog(dandanApiCacheC, "max waiting exceeds limit", "path", r.URL.Path, "cacheKey", key)
			web.ResponseJSON(w, http.StatusTooManyRequests, map[string]string{"message": "too many requests"})
			return
		}

		// 使用 singleflight 阻塞相同并发查询，只允许一个请求进行实际查询，其他则阻塞等待那一个实际请求返回并共享结果
		_, _, shared := cache.g.Do(key, func() (any, error) {
			rr := &responseRecorder{StatusRecorder: &web.StatusRecorder{ResponseWriter: w}}
			next.ServeHTTP(rr, r)
			if rr.Status == http.StatusOK {
				cacheData := rr.body.Bytes()
				cacheDuration := time.Duration(config.GetDandan().CacheTimeout * 1e9)
				if success := cache.cache.SetWithTTL(key, cacheData, int64(len(cacheData)), cacheDuration); !success {
					utils.ErrorLog(dandanApiCacheC, "cache set failed", "path", r.URL.Path, "cacheKey", key)
				}
			}
			return nil, nil
		})
		if shared {
			utils.DebugLog(dandanApiCacheC, "cache key shared", "path", r.URL.Path, "cacheKey", key)
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
