package iqiyi

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const xorKey = 0x75706971676c
const segmentInterval = 60
const segmentSalt = "cbzuw1259a"
const signSecret = "howcuteitis"
const signKey = "secret_key"

// 字符串tvid 转换为数字tvid
func parseToNumberId(id string) int64 {
	num, err := strconv.ParseInt(id, 36, 64)
	if err != nil {
		return 0
	}
	numBinary := strconv.FormatInt(num, 2)
	keyBinary := strconv.FormatInt(int64(xorKey), 2)

	numBits := reverseString(numBinary)
	keyBits := reverseString(keyBinary)

	maxLen := len(numBits)
	if len(keyBits) > maxLen {
		maxLen = len(keyBits)
	}

	resultBits := make([]byte, 0, maxLen)

	for i := 0; i < maxLen; i++ {
		var numBit, keyBit byte = '0', '0'
		if i < len(numBits) {
			numBit = numBits[i]
		}
		if i < len(keyBits) {
			keyBit = keyBits[i]
		}
		if numBit == keyBit {
			resultBits = append(resultBits, '0')
		} else {
			resultBits = append(resultBits, '1')
		}
	}

	finalBinary := reverseBytes(resultBits)

	if len(finalBinary) == 0 {
		return 0
	}
	val, _ := strconv.ParseInt(string(finalBinary), 2, 64)
	if val < 900000 {
		val = 100 * (val + 900000)
	}
	return val
}

func reverseString(s string) string {
	r := []byte(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func reverseBytes(b []byte) []byte {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return b
}

func buildSegmentUrl(tvId int64, segment int) string {
	// https://cmts.iqiyi.com/bullet/11/00/103411100_60_1_d5a87c30.br

	// build path
	path1 := "00"
	path2 := "00"
	tvIdStr := strconv.FormatInt(tvId, 10)
	l := len(tvIdStr)
	if l >= 4 {
		path1 = tvIdStr[l-4 : l-2]
	}
	if l >= 2 {
		path2 = tvIdStr[l-2:]
	}

	// build hash
	input := fmt.Sprintf("%s_%d_%d%s", tvIdStr, segmentInterval, segment, segmentSalt)
	sum := md5.Sum([]byte(input))
	hash := fmt.Sprintf("%x", sum)
	if len(hash) >= 8 {
		hash = hash[len(hash)-8:]
	}
	segmentStr := strconv.FormatInt(int64(segment), 10)
	api := fmt.Sprintf("https://cmts.iqiyi.com/bullet/%s/%s/%s_%d_%s_%s.br", path1, path2, tvIdStr, segmentInterval, segmentStr, hash)

	return api
}

func (c *client) sign(params url.Values) string {
	signParams := map[string]string{}
	var keys []string
	for k, v := range params {
		if k == "sign" {
			continue
		}
		signParams[k] = v[0]
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts = make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, signParams[k]))
	}

	hash := md5.Sum([]byte(fmt.Sprintf("%s&%s=%s", strings.Join(parts, "&"), signKey, signSecret)))
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}
