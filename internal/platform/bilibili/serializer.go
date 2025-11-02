package bilibili

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"fmt"
	"strconv"
	"strings"
)

type xmlSerializer struct{}

func (c *xmlSerializer) Type() string {
	return danmaku.XMLSerializer
}

type assSerializer struct{}

func (c *xmlSerializer) Serialize(d *danmaku.SerializerData) (interface{}, error) {
	if d.Data == nil {
		return nil, fmt.Errorf("ep%v danmaku is nil", d.EpisodeId)
	}

	// 合并重复弹幕
	var source = d.Data
	mergedMills := config.GetConfig().GetPlatformConfig(danmaku.Bilibili).MergeDanmakuInMills
	if mergedMills > 0 {
		source = danmaku.MergeDanmaku(source, mergedMills, d.DurationInMills)
	}

	var data = make([]danmaku.DataXMLDanmaku, len(source))
	// <d p="2.603,1,25,16777215,[bilibili]">看看 X2</d>
	// 第几秒/弹幕类型/字体大小/颜色
	for i, v := range source {
		var attr = []string{
			strconv.FormatFloat(float64(v.OffsetMills)/1000, 'f', 2, 64),
			strconv.FormatInt(int64(v.Mode), 10),
			strconv.FormatInt(int64(v.FontSize), 10),
			strconv.FormatInt(int64(v.Color), 10),
			fmt.Sprintf("[%s]", danmaku.Bilibili),
		}
		d := danmaku.DataXMLDanmaku{
			Attributes: strings.Join(attr, ","),
			Content:    v.Content,
		}
		data[i] = d
	}

	xml := danmaku.DataXML{
		ChatServer:     "chat.bilibili.com",
		ChatID:         d.SeasonId + "_" + d.EpisodeId,
		Mission:        0,
		MaxLimit:       2000,
		Source:         "k-v",
		SourceProvider: danmaku.Bilibili,
		DataSize:       len(source),
		Danmaku:        data,
	}

	return &xml, nil
}
