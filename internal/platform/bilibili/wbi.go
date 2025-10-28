package bilibili

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

func (c *client) setToken() error {
	keyUrl := "https://api.bilibili.com/x/web-interface/nav"
	req, err := http.NewRequest(http.MethodGet, keyUrl, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Cookie", c.common.Cookie)
	resp, err := c.common.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var nav navInfo
	err = json.NewDecoder(resp.Body).Decode(&nav)
	if err != nil {
		return err
	}
	if nav.Code != 0 {
		return errors.New(fmt.Sprintf("get nav fail: %v %s", nav.Code, nav.Message))
	}
	matchImg := tokenRegex.FindStringSubmatch(nav.Data.WbiImg.ImgUrl)
	matchSub := tokenRegex.FindStringSubmatch(nav.Data.WbiImg.SubUrl)
	if len(matchImg) <= 1 || len(matchSub) <= 1 {
		return errors.New(fmt.Sprintf("wrong img url token %s", nav.Data.WbiImg.ImgUrl))
	}
	c.token.imgKey = matchImg[1]
	c.token.subKey = matchSub[1]

	return nil
}

var tokenRegex = regexp.MustCompile(`bfs/wbi/(.{32})\..{3}`)

type navInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		WbiImg struct {
			ImgUrl string `json:"img_url"`
			SubUrl string `json:"sub_url"`
		} `json:"wbi_img"`
	} `json:"data"`
}

type tokenKey struct {
	subKey, imgKey string
	lastUpdateTime time.Time
}

func (c *client) sign(values url.Values) (url.Values, error) {
	tokenExpire := time.Since(c.token.lastUpdateTime).Hours() > 24
	if c.token.imgKey == "" || c.token.subKey == "" || tokenExpire {
		err := c.setToken()
		if err != nil {
			return nil, err
		}
		c.token.lastUpdateTime = time.Now()
	}

	values = removeUnwantedChars(values, '!', '\'', '(', ')', '*')
	values.Set("wts", strconv.FormatInt(time.Now().Unix(), 10))

	var mixin [32]byte
	wbi := c.token.imgKey + c.token.subKey
	for i := range mixin {
		mixin[i] = wbi[mixinKeyEncTab[i]]
	}
	hash := md5.Sum([]byte(values.Encode() + string(mixin[:]))) // Calculate w_rid
	values.Set("w_rid", hex.EncodeToString(hash[:]))

	return values, nil
}

var mixinKeyEncTab = [...]int{
	46, 47, 18, 2, 53, 8, 23, 32,
	15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19,
	29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61,
	26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63,
	57, 62, 11, 36, 20, 34, 44, 52,
}

func removeUnwantedChars(v url.Values, chars ...byte) url.Values {
	b := []byte(v.Encode())
	for _, c := range chars {
		b = bytes.ReplaceAll(b, []byte{c}, nil)
	}
	s, err := url.ParseQuery(string(b))
	if err != nil {
		panic(err)
	}
	return s
}
