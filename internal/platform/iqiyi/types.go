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
