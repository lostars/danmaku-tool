package danmaku

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/utils"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const managerUtilC = "manager_util"

func MergeDanmaku(dms []*StandardDanmaku, mergedInMills int64, durationInMills int64) []*StandardDanmaku {
	var start = time.Now()
	utils.DebugLog(managerUtilC, "danmaku size merge start", "size", len(dms))
	if mergedInMills <= 0 {
		utils.DebugLog(managerUtilC, "danmaku size merge no merge mills set")
		return dms
	}
	var initBuckets int64
	if durationInMills > 0 {
		initBuckets = durationInMills/mergedInMills + 1
	} else {
		utils.DebugLog(managerUtilC, "danmaku size merge no duration mills set")
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

	utils.DebugLog(managerUtilC, "danmaku size merge end", "size", len(result), "cost_ms", time.Since(start).Milliseconds())

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

var SeriesRegex = regexp.MustCompile(`(.*)\sS(\d{1,3})E(\d{1,3})$`)
var ChineseNumber = "一|二|三|四|五|六|七|八|九|十|十一|十二|十三|十四|十五|十六|十七|十八|十九|二十"
var ChineseNumberSlice = strings.Split(ChineseNumber, "|")
var MarkRegex = regexp.MustCompile(`[\p{P}\p{S}]`)
var SeasonTitleMatch = regexp.MustCompile(`第\s*(\d{1,2}|` + ChineseNumber + `)\s*季`)
var MatchLanguage = regexp.MustCompile(`(特别|普通话|粤配|中配|中文|粤语)\(版|篇\)*$`)
var MatchSpecials = regexp.MustCompile(`(特别)篇$`)
var MatchKeyword = regexp.MustCompile(`<em(\sclass="keyword")*>(.*?)</em>`)
var EpBlacklistRegex = regexp.MustCompile(`PV|专访|预告|花絮|彩蛋|高光.*\d*`)

func InvalidEpTitle(title string) bool {
	return EpBlacklistRegex.MatchString(title)
}

func ClearTitle(title string) string {
	var clearTitle = utils.StripHTMLTags(title)
	clearTitle = MarkRegex.ReplaceAllLiteralString(clearTitle, "")
	return clearTitle
}

func ClearTitleAndSeason(title string) string {
	clearTitle := strings.ReplaceAll(title, " ", "")
	return SeasonTitleMatch.ReplaceAllLiteralString(ClearTitle(clearTitle), "")
}

// MatchSeason 匹配标题中的季信息 返回数字季 没匹配上则返回-1
func MatchSeason(title string) int {
	seasonMatches := SeasonTitleMatch.FindStringSubmatch(title)
	if len(seasonMatches) <= 1 {
		return -1
	}
	s, err := strconv.ParseInt(seasonMatches[1], 10, 64)
	if err == nil {
		return int(s)
	} else {
		return GetNumberSeasonFromChinese(seasonMatches[1])
	}
}

func GetNumberSeasonFromChinese(chineseNumber string) int {
	for i, v := range ChineseNumberSlice {
		if v == chineseNumber {
			return i + 1
		}
	}
	return -1
}

// MatchTitle 标题匹配包含两部分：季信息 和 标题本身
func (p MatchParam) MatchTitle(title string) bool {
	if title == "" {
		return false
	}
	// 检查em标签是否有命中搜索词
	if p.CheckEm {
		emMatches := MatchKeyword.FindStringSubmatch(title)
		if len(emMatches) <= 2 {
			return false
		}
		if len(emMatches) > 2 {
			if emMatches[2] == "" {
				return false
			}
			title = ClearTitle(title)
		}
	}
	matchMode := string(p.Mode)
	// 黑名单 正则匹配替换
	if config.GetConfig().Tokenizer.Enable && config.GetConfig().Tokenizer.Blacklist != nil {
		for _, r := range config.GetConfig().Tokenizer.Blacklist {
			re, err := regexp.Compile(r.Regex)
			if err != nil {
				continue
			}
			// 全平台
			noneMatchPlatform := r.Platform == ""
			// 特定平台
			matchPlatform := r.Platform != "" && r.Platform == string(p.Platform)
			if (noneMatchPlatform || matchPlatform) && re.MatchString(title) {
				// 更改后续匹配模式
				if r.Mode != "" {
					matchMode = r.Mode
				}
				title = re.ReplaceAllLiteralString(title, r.Replacement)
				// 只匹配一次
				break
			}
		}
	}
	// 如果是搜索模式，则匹配到命中搜索词结束
	if p.Mode == Search {
		lowerClearTitle := strings.ToLower(ClearTitleAndSeason(title))
		targetLowerTitle := strings.ToLower(ClearTitleAndSeason(p.Title))
		return strings.Contains(lowerClearTitle, targetLowerTitle)
	}
	// 处理 S0
	if p.SeasonId == 0 {
		return MatchSpecials.MatchString(title)
	}
	// 语言版本处理 直接过滤掉
	// 不同平台语言标题不一致，有些会把非原版语言添加到标题，有些会把原版添加到标题（日语版）
	if !MatchLanguage.MatchString(p.Title) {
		if MatchLanguage.MatchString(title) {
			return false
		}
	}
	// 优先匹配季信息
	if p.SeasonId > 0 {
		season := MatchSeason(title)
		if season < 0 {
			if p.SeasonId == 1 {
				// 很多剧集是没有把第一季加到标题上的，如果未匹配出第一季那就继续处理
			} else {
				return false
			}
		} else {
			if season != p.SeasonId {
				return false
			}
		}
	}

	// 最后清理标题 再次匹配
	lowerClearTitle := strings.ToLower(ClearTitleAndSeason(title))
	targetLowerTitle := strings.ToLower(ClearTitleAndSeason(p.Title))
	switch matchMode {
	case Ignore:
		return true
	case Equals:
		// 使用替换方式，方式标题重复出现
		result := strings.ReplaceAll(lowerClearTitle, targetLowerTitle, "")
		return result == ""
	case Contains:
		return strings.Contains(lowerClearTitle, targetLowerTitle)
	}
	return false
}

// MatchMode 用于 MatchTitle 最后的匹配方式
type MatchMode string

const (
	Equals   = "equals"
	Contains = "contains"
	Ignore   = "ignore"
	Search   = "search"
)

func (p MatchParam) MatchYear(year int) bool {
	if p.ProductionYear > 0 {
		return year == p.ProductionYear
	}
	return true
}

func (p MatchParam) MatchYearString(year string) (int, bool) {
	y, e := strconv.ParseInt(year, 10, 64)
	if e != nil {
		return 0, false
	}
	if p.MatchYear(int(y)) {
		return int(y), true
	}
	return int(y), false
}

func InitPlatformClient(c *PlatformClient, platform Platform) error {
	conf := config.GetPlatformConfig(string(platform))
	if conf == nil || conf.Name == "" {
		return fmt.Errorf("[%s] is not configured", platform)
	}
	if conf.Priority < 0 {
		utils.InfoLog(managerUtilC, fmt.Sprintf("[%s] is disabled", platform))
		return nil
	}

	c.Cookie = conf.Cookie
	c.MaxWorker = conf.MaxWorker
	if c.MaxWorker <= 0 {
		c.MaxWorker = defaultMaxWorker
	}
	var timeout = conf.Timeout
	if timeout <= 0 {
		timeout = defaultTimeoutInSeconds
	}
	c.HttpClient = &http.Client{Timeout: time.Duration(timeout * 1e9)}

	return nil
}

const (
	defaultMaxWorker        = 4
	defaultTimeoutInSeconds = 30
)
