package utils

import (
	"io"
	"regexp"
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
