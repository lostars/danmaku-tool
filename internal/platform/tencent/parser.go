package tencent

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type xmlParser struct {
	// 弹幕数据
	danmaku     []*danmaku.StandardDanmaku
	vid         string
	danmakuLock sync.Mutex
	duration    int64
	platform    danmaku.Platform
}

func (c *xmlParser) Parse() (*danmaku.DataXML, error) {
	if c.danmaku == nil {
		return nil, danmaku.PlatformError(danmaku.Tencent, "danmaku is nil")
	}

	var source = c.danmaku
	mergedMills := config.GetConfig().GetPlatformConfig(danmaku.Tencent).MergeDanmakuInMills
	if mergedMills > 0 {
		source = danmaku.MergeDanmaku(source, mergedMills, c.duration)
	}

	var data = make([]danmaku.DataXMLDanmaku, len(source))
	// <d p="2.603,1,25,16777215,[tencent]">看看 X2</d>
	// 第几秒/弹幕类型/字体大小/颜色
	for i, v := range source {
		var attr = []string{
			strconv.FormatFloat(float64(v.Offset)/1000, 'f', 2, 64),
			strconv.FormatInt(int64(v.Mode), 10),
			"25", // 腾讯视频弹幕没有字体大小，固定25
			strconv.FormatInt(int64(v.Color), 10),
			fmt.Sprintf("[%s]", c.platform),
		}
		d := danmaku.DataXMLDanmaku{
			Attributes: strings.Join(attr, ","),
			Content:    v.Content,
		}
		data[i] = d
	}

	xml := danmaku.DataXML{
		ChatServer:     "chat.v.qq.com",
		ChatID:         c.vid,
		Mission:        0,
		MaxLimit:       2000,
		Source:         "k-v",
		SourceProvider: danmaku.Tencent,
		DataSize:       len(source),
		Danmaku:        data,
	}

	return &xml, nil
}
