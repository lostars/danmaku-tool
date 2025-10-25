package utils

import (
	"regexp"
)

func StripHTMLTags(htmlStr string) string {
	re := regexp.MustCompile("<[^>]*>")
	cleanText := re.ReplaceAllString(htmlStr, "")
	return cleanText
}
