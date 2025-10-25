package danmaku

import (
	"danmu-tool/internal/utils"
	"strconv"
	"strings"
	"time"
)

func MergeDanmaku(dms []*StandardDanmaku, mergedInMills int64, durationInMills int64) []*StandardDanmaku {
	var start = time.Now().Nanosecond()
	logger := utils.GetComponentLogger("manager")
	logger.Debug("danmaku size before merge", "size", len(dms))
	var totalBuckets = durationInMills/mergedInMills + 1
	buckets := make(map[int64]map[string]bool, totalBuckets)
	var result = make([]*StandardDanmaku, 0, len(dms))

	for _, d := range dms {
		bid := d.Offset / mergedInMills // 所属时间桶

		if _, ok := buckets[bid]; !ok {
			// 预估长度
			buckets[bid] = make(map[string]bool, int64(len(dms))/totalBuckets+1)
		}

		// 检查当前桶和前一个桶是否出现过（跨桶重复处理）
		if buckets[bid][d.Content] || buckets[bid-1][d.Content] {
			continue
		}

		result = append(result, d)
		buckets[bid][d.Content] = true
	}

	var end = time.Now().Nanosecond()
	logger.Debug("danmaku size before merge", "size", len(result))
	logger.Debug("danmaku merge cost", "duration", end-start)

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
