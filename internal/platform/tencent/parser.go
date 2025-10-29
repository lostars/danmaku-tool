package tencent

import (
	"danmu-tool/internal/danmaku"
)

type xmlParser struct {
	// 弹幕数据
	danmaku         []*danmaku.StandardDanmaku
	vid             string
	durationInMills int64
}

func (c *xmlParser) Parse() (*danmaku.DataXML, error) {
	if c.danmaku == nil {
		return nil, danmaku.PlatformError(danmaku.Tencent, "danmaku is nil")
	}

	xml := danmaku.DataXML{
		ChatServer:     "chat.v.qq.com",
		ChatID:         c.vid,
		Mission:        0,
		MaxLimit:       2000,
		Source:         "k-v",
		SourceProvider: danmaku.Tencent,
		DataSize:       len(c.danmaku),
		Danmaku:        danmaku.NormalConvert(c.danmaku, danmaku.Tencent, c.durationInMills),
	}

	return &xml, nil
}
