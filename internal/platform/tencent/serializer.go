package tencent

import (
	"danmaku-tool/internal/danmaku"
	"fmt"
)

type xmlSerializer struct{}

func (c *xmlSerializer) Type() string {
	return danmaku.XMLSerializer
}

type assSerializer struct{}

func (c *xmlSerializer) Serialize(d *danmaku.SerializerData) (interface{}, error) {
	if d.Data == nil {
		return nil, fmt.Errorf("danmaku is nil")
	}

	xml := danmaku.DataXML{
		ChatServer:     "chat.v.qq.com",
		ChatID:         d.EpisodeId,
		Mission:        0,
		MaxLimit:       2000,
		Source:         "k-v",
		SourceProvider: danmaku.Tencent,
		DataSize:       len(d.Data),
		Danmaku:        danmaku.NormalConvert(d.Data, danmaku.Tencent, d.DurationInMills),
	}

	return &xml, nil
}
