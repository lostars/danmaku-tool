package danmaku

import (
	"danmu-tool/internal/config"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

var embyYearRegex = regexp.MustCompile(`^(\d{4})-\d{2}-\d{2}T`)

type EmbySearchResult struct {
	TotalRecordCount int `json:"TotalRecordCount"`
	Items            []struct {
		Name string `json:"Name"`
		Id   string `json:"Id"`
		// Continuing/Ended
		Status         string `json:"Status"`
		Type           string `json:"Type"`
		ProductionYear int    `json:"ProductionYear"`
		EndDate        string `json:"EndDate"`
	} `json:"Items"`
}

var embyClient = http.Client{
	Timeout: time.Second * 10,
}

func SearchEmby(fileName string) (*EmbySearchResult, error) {
	types := "Movie"
	if SeriesRegex.MatchString(fileName) {
		types = "Series"
	}
	params := url.Values{
		"Fields":           {"ProductionYear", "Status", "EndDate", "BasicSyncInfo"},
		"IncludeItemTypes": {types},
		"Recursive":        {"true"},
		"SearchTerm":       {fileName},
		"Limit":            {"50"},
		"SortBy":           {"SortName"},
		"SortOrder":        {"Ascending"},
	}

	embyConfig := config.GetConfig().Emby
	api := fmt.Sprintf("%s/emby/Users/%s/Items?%s", embyConfig.Url, embyConfig.User, params.Encode())

	req, _ := http.NewRequest(http.MethodGet, api, nil)
	req.Header.Set("X-Emby-Token", embyConfig.Token)
	req.Header.Set("X-Emby-Client", "danmaku-tool")
	req.Header.Set("X-Emby-Device-Name", "danmaku-tool")

	resp, err := embyClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result EmbySearchResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
