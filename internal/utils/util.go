package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

func StripHTMLTags(htmlStr string) string {
	re := regexp.MustCompile("<[^>]*>")
	cleanText := re.ReplaceAllString(htmlStr, "")
	return cleanText
}

func SafeClose(c io.Closer) {
	if e := c.Close(); e != nil {
		ErrorLog("utils", e.Error())
	}
}

func SafeDecodeOkResp(resp *http.Response, v any) error {
	defer SafeClose(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("error http status: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return err
	}
	return nil
}

func NoneNumberNorEmpty(str string) bool {
	if str == "" {
		return false
	}
	_, err := strconv.ParseInt(str, 10, 64)
	return err != nil
}
