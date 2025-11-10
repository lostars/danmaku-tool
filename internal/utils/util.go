package utils

import (
	"fmt"
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
		fmt.Println(e.Error())
	}
}
