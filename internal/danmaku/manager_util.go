package danmaku

import (
	"danmu-tool/internal/utils"
	"strconv"
	"strings"
	"time"
)

func MergeDanmaku(dms []*StandardDanmaku, mergedInMills int64, durationInMills int64) []*StandardDanmaku {
	var start = time.Now()
	logger := utils.GetComponentLogger("manager-util")
	logger.Debug("danmaku size merge start", "size", len(dms))
	if mergedInMills <= 0 {
		logger.Debug("danmaku size merge no merge mills set")
		return dms
	}
	var initBuckets int64
	if durationInMills > 0 {
		initBuckets = durationInMills/mergedInMills + 1
	} else {
		logger.Debug("danmaku size merge no duration mills set")
		initBuckets = 7200 // 2h
	}
	buckets := make(map[int64]map[string]bool, initBuckets)
	var result = make([]*StandardDanmaku, 0, len(dms))

	for _, d := range dms {
		bid := d.Offset / mergedInMills // 所属时间桶

		if _, ok := buckets[bid]; !ok {
			// 预估长度
			buckets[bid] = make(map[string]bool, int64(len(dms))/initBuckets+1)
		}

		// 检查当前桶和前一个桶是否出现过（跨桶重复处理）
		if buckets[bid][d.Content] || buckets[bid-1][d.Content] {
			continue
		}

		result = append(result, d)
		buckets[bid][d.Content] = true
	}

	logger.Debug("danmaku size merge end", "size", len(result), "cost_ms", time.Since(start).Milliseconds())

	return result
}

func (d *StandardDanmaku) GenDandanAttribute(text ...string) string {
	var attr = []string{
		strconv.FormatFloat(float64(d.Offset)/1000, 'f', 2, 64),
		strconv.FormatInt(int64(d.Mode), 10),
		strconv.FormatInt(int64(d.Color), 10),
		// 该字段在dandan api中为用户id，注意SenPlayer中该字段必须返回，且为int
		// 但在某些web js插件中该字段又未处理，所以这里依旧按照dandan api定义，返回0
		"0",
	}
	attr = append(attr, text...)
	return strings.Join(attr, ",")
}
