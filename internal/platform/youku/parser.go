package youku

import (
	"danmu-tool/internal/danmaku"
	"fmt"
)

type xmlParser struct {
	// 弹幕数据
	danmaku           []*danmaku.StandardDanmaku
	vid               string
	durationInSeconds int64
}

func (c *xmlParser) Parse() (*danmaku.DataXML, error) {
	if c.danmaku == nil {
		return nil, fmt.Errorf("danmaku is nil")
	}

	xml := danmaku.DataXML{
		ChatServer:     "chat.v.youku.com",
		ChatID:         c.vid,
		Mission:        0,
		MaxLimit:       2000,
		Source:         "k-v",
		SourceProvider: danmaku.Youku,
		DataSize:       len(c.danmaku),
		Danmaku:        danmaku.NormalConvert(c.danmaku, danmaku.Youku, c.durationInSeconds*1000),
	}

	return &xml, nil
}
