package iqiyi

type VideoBaseInfoResult struct {
	Code string `json:"code"` // A00000 成功
	Data struct {
		TVId        int64  `json:"tvId"`
		AlbumId     int64  `json:"albumId"`
		AlbumName   string `json:"albumName"`
		Name        string `json:"name"`        // 武林外传第62集
		VideoCount  int    `json:"videoCount"`  // 剧集集数
		DurationSec int    `json:"durationSec"` // 视频长度 s
		Order       int    `json:"order"`       // 排序 和集数对应
	} `json:"data"`
}

func (v *VideoBaseInfoResult) success() bool {
	return v.Code == "A00000"
}

type SearchResult struct {
	Code int `json:"code"`
	Data struct {
		RealQuery string           `json:"realQuery"`
		Terms     []string         `json:"terms"`
		Templates []SearchTemplate `json:"templates"`
	} `json:"data"`
}

type SearchTemplate struct {
	S3        string `json:"s3"`       // 剧集类型展示 template说明
	Template  int    `json:"template"` // 101=剧集 电影=103
	AlbumInfo struct {
		SiteId       string `json:"siteId"`       // 站点id miguvideo iqiyi 可以用于过滤非本站视频
		Introduction string `json:"introduction"` // 简介
		// 电影存放的是tvId 剧集则是第一集的tvId 格式：剧集里这个字段一样
		// qips://tvid=1625689363120100;vid=401ee768727dbc42c41286bfa24c8715;ischarge=true;vtype=0;ht=2;lt=2;
		// 剧集的playUrl 能解析到 albumId
		// qips://tvid=7726962800722900;vid=42b9d564e33c95534a44826bc8f246d3;ischarge=false;vtype=0;fid=1532638122874101;ht=0;lt=3;albumid=1532638122874101;
		PlayUrl  string `json:"playUrl"`
		Subtitle string `json:"subtitle"` // 年份 2002
		Title    string `json:"title"`

		// 以下是剧集返回
		TotalNumber int `json:"totalNumber"` // 集数
		Videos      []struct {
			DurationInMills int    `json:"duration"` // ms时长
			Number          string `json:"number"`   // 剧集编号 1
			Subtitle        string `json:"subtitle"` // 集副标题
			Title           string `json:"title"`
			PlayUrl         string `json:"playUrl"`
		} `json:"videos"`

		// 以下是单个视频返回
		DurationInMills int `json:"duration"` // ms时长 单个视频才返回
	} `json:"albumInfo"`
}

func (v *SearchResult) success() bool {
	return v.Code == 0
}
