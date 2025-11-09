package danmaku

import (
	"danmaku-tool/internal/config"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type EmbySearchResult struct {
	TotalRecordCount int         `json:"TotalRecordCount"`
	Items            []*EmbyItem `json:"Items"`
}

type EmbyItem struct {
	Name string `json:"Name"`
	Id   string `json:"Id"`
	// Continuing/Ended
	Status string `json:"Status"`
	// Season/Series/Movie
	Type           string `json:"Type"`
	ProductionYear int    `json:"ProductionYear"`
	EndDate        string `json:"EndDate"`

	// 季/集
	IndexNumber int `json:"IndexNumber"`
	// 父id，season id
	ParentIndexNumber int `json:"ParentIndexNumber"`
}

var embyClient = http.Client{
	Timeout: time.Second * 10,
}

const (
	EmbyMovie  = "Movie"
	EmbySeries = "Series"
)

func SearchEmby(fileName string, ssId int) (*EmbySearchResult, error) {
	types := EmbyMovie
	if ssId >= 0 {
		types = EmbySeries
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

	return doEmbyGet[EmbySearchResult](api)
}

func doEmbyGet[T any](api string) (*T, error) {

	req, _ := http.NewRequest(http.MethodGet, api, nil)
	req.Header.Set("X-Emby-Token", config.GetConfig().Emby.Token)
	req.Header.Set("X-Emby-Client", "danmaku-tool")
	req.Header.Set("X-Emby-Device-Name", "danmaku-tool")

	resp, err := embyClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result T
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func GetSeasons(id string, recursive bool) (*EmbySearchResult, error) {
	embyConfig := config.GetConfig().Emby
	params := url.Values{
		// 季节也有年份信息，一定要带上查询
		"Fields":    {"ProductionYear", "Status", "EndDate", "BasicSyncInfo"},
		"UserId":    {embyConfig.User},
		"Recursive": {strconv.FormatBool(recursive)},
	}

	api := fmt.Sprintf("%s/emby/Shows/%s/Seasons?%s", embyConfig.Url, id, params.Encode())

	return doEmbyGet[EmbySearchResult](api)
}
