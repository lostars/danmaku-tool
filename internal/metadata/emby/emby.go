package emby

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type client struct {
	httpClient       *http.Client
	user, token, url string
}

func (c client) Priority() int {
	return 10
}

func (c client) Source() danmaku.Source {
	return danmaku.Emby
}

func init() {
	danmaku.Register(&client{})
}

func (c client) AsyncInit() error {
	conf := config.GetConfig().Emby
	if conf.Token != "" && conf.User != "" && conf.Url != "" {
		c.httpClient = &http.Client{Timeout: 10 * time.Second}
		c.user = conf.User
		c.token = conf.Token
		c.url = conf.Url
		danmaku.RegisterMetadata(c)
	} else {
		utils.InfoLog(danmaku.Emby, fmt.Sprintf("[%s] is not configured", danmaku.Emby))
		return nil
	}

	// test api
	api := fmt.Sprintf("%s/emby/Users/%s", c.url, c.user)
	result := map[string]interface{}{}
	if err := c.doEmbyGet(api, &result); err != nil {
		utils.ErrorLog(danmaku.Emby, fmt.Sprintf("emby api test failed: %s", err.Error()))
	}
	return nil
}

func (c client) ServerInit() error {
	return nil
}

func (c client) Year(name string, seasonId string) (year int, err error) {
	types := Movie
	if seasonId != "" {
		types = Series
	}
	params := url.Values{
		"Fields":           {"ProductionYear", "Status", "EndDate", "BasicSyncInfo"},
		"IncludeItemTypes": {types},
		"Recursive":        {"true"},
		"SearchTerm":       {name},
		"Limit":            {"50"},
		"SortBy":           {"SortName"},
		"SortOrder":        {"Ascending"},
	}
	api := fmt.Sprintf("%s/emby/Users/%s/Items?%s", c.url, c.user, params.Encode())

	var result SearchResult
	if err = c.doEmbyGet(api, &result); err != nil {
		return
	}
	if len(result.Items) < 1 {
		return
	}
	if len(result.Items) > 1 {
		utils.WarnLog(danmaku.Emby, fmt.Sprintf("[%s] match more than 1 media", name))
	}
	item := result.Items[0]
	switch item.Type {
	case Movie:
		year = item.ProductionYear
	case Series:
		if season, e := c.GetSeasons(item.Id, false); e == nil && len(season.Items) > 1 {
			for _, s := range season.Items {
				if strconv.FormatInt(int64(s.IndexNumber), 10) == seasonId {
					year = s.ProductionYear
					break
				}
			}
		} else {
			year = item.ProductionYear
		}
	}
	return
}

func (c client) Episodes(id string) ([]*danmaku.MediaEpisode, error) {
	season, e := c.GetSeasons(id, true)
	if e != nil {
		return nil, e
	}
	var result = make([]*danmaku.MediaEpisode, 0, len(season.Items))
	for _, ep := range season.Items {
		if ep.ParentIndexNumber == nil {
			continue
		}
		result = append(result, &danmaku.MediaEpisode{
			Title:     ep.SeriesName,
			EpisodeId: strconv.FormatInt(int64(ep.IndexNumber), 10),
			Id:        ep.Id,
			SeasonId:  *ep.ParentIndexNumber,
		})
	}

	return result, nil
}

type SearchResult struct {
	TotalRecordCount int     `json:"TotalRecordCount"`
	Items            []*Item `json:"Items"`
}

type Item struct {
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
	// 父id，season id 0代表S0 这里用指针 nil表示无parentId
	ParentIndexNumber *int   `json:"ParentIndexNumber"`
	SeriesName        string `json:"SeriesName"`
}

const (
	Movie  = "Movie"
	Series = "Series"
)

func (c client) doEmbyGet(api string, v any) error {

	req, _ := http.NewRequest(http.MethodGet, api, nil)
	req.Header.Set("X-Emby-Token", c.token)
	req.Header.Set("X-Emby-Client", "danmaku-tool")
	req.Header.Set("X-Emby-Device-Name", "danmaku-tool")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer utils.SafeClose(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("emby failed: %s, %s", api, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return err
	}
	return nil
}

func (c client) GetSeasons(id string, recursive bool) (*SearchResult, error) {
	embyConfig := config.GetConfig().Emby
	params := url.Values{
		// 季节也有年份信息，一定要带上查询
		"Fields":    {"ProductionYear", "Status", "EndDate", "BasicSyncInfo"},
		"UserId":    {embyConfig.User},
		"Recursive": {strconv.FormatBool(recursive)},
	}

	api := fmt.Sprintf("%s/emby/Shows/%s/Seasons?%s", embyConfig.Url, id, params.Encode())

	var result SearchResult
	return &result, c.doEmbyGet(api, &result)
}
