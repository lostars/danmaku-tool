package youku

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func (c *client) refreshToken() {
	// cna https://log.mmstat.com/eg.js etag "C2CHIZvOsxUCAQAAAADMJgVh"
	cnaUrl := "https://log.mmstat.com/eg.js"
	cnaReq, _ := http.NewRequest(http.MethodGet, cnaUrl, nil)
	cnaResp, e := c.common.DoReq(cnaReq)
	if e != nil {
		return
	}
	defer cnaResp.Body.Close()
	if cnaResp.StatusCode != http.StatusOK {
		return
	}
	etags := cnaResp.Header.Values("etag")
	if etags == nil || len(etags) < 1 {
		return
	}
	c.cna = etags[0]

	api := "https://acs.youku.com/h5/mtop.com.youku.aplatform.weakget/1.0/?jsv=2.5.1&appKey=24679788"
	req, _ := http.NewRequest(http.MethodGet, api, nil)
	req.Header.Set("cookie", "cna="+c.cna)
	resp, err := c.common.DoReq(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	cookies := resp.Header.Values("set-cookie")
	if cookies == nil || len(cookies) < 2 {
		return
	}
	for _, cookie := range cookies {
		tkMatches := tkRegex.FindStringSubmatch(cookie)
		if len(tkMatches) > 1 {
			c.token = tkMatches[1]
		}
		encTkMatches := encTkRegex.FindStringSubmatch(cookie)
		if len(encTkMatches) > 1 {
			c.tokenEnc = encTkMatches[1]
		}
	}
	c.tkLastUpdate = time.Now()
}

var tkRegex = regexp.MustCompile(`_m_h5_tk=([a-z0-9]{32}_[0-9]{13});`)
var encTkRegex = regexp.MustCompile(`_m_h5_tk_enc=([a-z0-9]{32});`)

const salt = "MkmC9SoIw6xCkSKHhJ7b5D2r51kBiREr"

func generateTokenSign(token, t, appKey, data string) string {
	tokenPart := token
	if len(token) >= 32 {
		tokenPart = token[:32]
	}

	parts := []string{tokenPart, t, appKey, data}
	s := strings.Join(parts, "&")

	h := md5.New()
	_, _ = io.WriteString(h, s)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func signPayload() (string, string) {
	msgMap := map[string]interface{}{
		"pid":   0,
		"ctype": 10004,
		"sver":  "3.1.0",
		"cver":  "v1.0",
		"ctime": time.Now().UnixMilli(),
		//"guid":   cna,
		//"vid": vid,
		//"mat":    mat,
		"mcount": 1,
		"type":   1,
	}
	msgJSON, _ := json.Marshal(msgMap)
	// "msg": "", "sign": ""
	msg := base64.StdEncoding.EncodeToString(msgJSON)

	h := md5.New()
	_, _ = io.WriteString(h, msg+salt)
	return msg, fmt.Sprintf("%x", h.Sum(nil))
}

func (c *client) setReq(req *http.Request) {
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	req.Header.Set("cookie", fmt.Sprintf("_m_h5_tk=%s;_m_h5_tk_enc=%s;cna=%s", c.token, c.tokenEnc, c.cna))
}

func (c *client) sign(params map[string]interface{}, api apiInfo) (url.Values, string) {
	if c.cna == "" || c.token == "" || c.tokenEnc == "" || time.Since(c.tkLastUpdate).Hours() >= 24 {
		c.refreshToken()
	}

	msg, sign := signPayload()

	params["msg"] = msg
	params["sign"] = sign
	payload, _ := json.Marshal(params)

	t := fmt.Sprintf("%d", time.Now().UnixMilli())
	tokenSign := generateTokenSign(c.token, t, api.key, string(payload))

	urlParams := url.Values{}
	urlParams.Set("jsv", "2.6.1")
	urlParams.Set("appKey", api.key)
	urlParams.Set("t", t)
	urlParams.Set("sign", tokenSign)
	urlParams.Set("api", api.api)
	urlParams.Set("v", "1.0")
	urlParams.Set("type", "originaljson")
	urlParams.Set("dataType", "jsonp")
	urlParams.Set("timeout", "20000")

	return urlParams, string(payload)
}

var (
	danmakuList = apiInfo{key: "24679788", api: "mopen.youku.danmu.list"}
	search      = apiInfo{key: "23774304", api: "mtop.youku.soku.yksearch"}
)

type apiInfo struct {
	key string
	api string
}
