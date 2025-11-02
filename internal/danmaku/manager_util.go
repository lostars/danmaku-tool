package danmaku

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/utils"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yanyiwu/gojieba"
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
		bid := d.OffsetMills / mergedInMills // 所属时间桶

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
		strconv.FormatFloat(float64(d.OffsetMills)/1000, 'f', 2, 64),
		strconv.FormatInt(int64(d.Mode), 10),
		strconv.FormatInt(int64(d.Color), 10),
		// 该字段在dandan api中为用户id，注意SenPlayer中该字段必须返回，且为int
		// 但在某些web js插件中该字段又未处理，所以这里依旧按照dandan api定义，返回0
		"0",
	}
	attr = append(attr, text...)
	return strings.Join(attr, ",")
}

const defaultUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"

func (p *PlatformClient) DoReq(req *http.Request) (*http.Response, error) {
	ua := config.GetConfig().UA
	if ua == "" {
		ua = defaultUA
	}
	req.Header.Set("User-Agent", ua)
	return p.HttpClient.Do(req)
}

func ClearTitle(title string) string {
	var clearTitle = utils.StripHTMLTags(title)
	clearTitle = strings.ReplaceAll(clearTitle, " ", "")
	clearTitle = MarkRegex.ReplaceAllLiteralString(clearTitle, "")
	seasonMatches := SeasonTitleMatch.FindStringSubmatch(clearTitle)
	if len(seasonMatches) > 1 {
		s, err := strconv.ParseInt(seasonMatches[1], 10, 64)
		if err == nil && int(s) < len(ChineseNumberSlice) && s >= 1 {
			clearTitle = strings.ReplaceAll(clearTitle, seasonMatches[1], ChineseNumberSlice[s-1])
		}
	}
	return clearTitle
}

func InitPlatformClient(platform Platform) (*PlatformClient, error) {
	conf := config.GetConfig().GetPlatformConfig(string(platform))
	if conf == nil || conf.Name == "" {
		return nil, fmt.Errorf("%s is not configured", platform)
	}
	if conf.Priority < 0 {
		return nil, fmt.Errorf("%s is disabled", platform)
	}

	c := &PlatformClient{}

	c.Cookie = conf.Cookie
	c.MaxWorker = conf.MaxWorker
	if c.MaxWorker <= 0 {
		c.MaxWorker = 8
	}
	var timeout = conf.Timeout
	if timeout <= 0 {
		timeout = 60
	}
	c.HttpClient = &http.Client{Timeout: time.Duration(timeout * 1e9)}
	c.Logger = utils.GetPlatformLogger(string(platform))

	return c, nil
}

type StringTokenizer struct {
	jieba *gojieba.Jieba
}

var Tokenizer = StringTokenizer{}

func (t *StringTokenizer) ServerInit() error {
	tokenizer := config.GetConfig().Tokenizer
	if !tokenizer.Enable {
		return nil
	}
	t.jieba = gojieba.NewJieba(config.JiebaDictTempDirs...)
	for _, w := range tokenizer.Words {
		if w != "" {
			t.jieba.AddWord(w)
		}
	}
	return nil
}

func init() {
	RegisterInitializer(&Tokenizer)
}

func (t *StringTokenizer) Finalize() error {
	if t.jieba != nil {
		t.jieba.Free()
	}
	return nil
}

func (t *StringTokenizer) Match(source, target string) bool {
	// 处理语言
	if !MatchLanguage.MatchString(target) {
		if MatchLanguage.MatchString(source) {
			return false
		}
	}

	tokenizer := config.GetConfig().Tokenizer
	if !tokenizer.Enable {
		return true
	}

	//分词匹配
	sourceTokens := t.jieba.Cut(source, true)
	targetTokens := t.jieba.Cut(target, true)
	count := 0
	for _, targetT := range targetTokens {
		for _, sourceT := range sourceTokens {
			if sourceT == targetT {
				count++
				break
			}
		}
	}
	// 媒体标题通常较短 默认全部匹配
	return len(targetTokens)-count == 0
}
