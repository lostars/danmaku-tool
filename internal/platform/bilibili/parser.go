package bilibili

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"fmt"
	"strconv"
	"strings"
)

type xmlParser struct {
	danmaku        []*danmaku.StandardDanmaku
	epId, seasonId int64
	// ep时长 ms
	epDuration int64
}

func (c *xmlParser) Parse() (*danmaku.DataXML, error) {
	if c.danmaku == nil {
		return nil, danmaku.PlatformError(danmaku.Bilibili, fmt.Sprintf("ep%v danmaku is nil", c.epId))
	}

	// 合并重复弹幕
	var source = c.danmaku
	mergedMills := config.GetConfig().GetPlatformConfig(danmaku.Bilibili).MergeDanmakuInMills
	if mergedMills > 0 {
		source = danmaku.MergeDanmaku(source, mergedMills, c.epDuration)
	}

	var data = make([]danmaku.DataXMLDanmaku, len(source))
	// <d p="2.603,1,25,16777215,[bilibili]">看看 X2</d>
	// 第几秒/弹幕类型/字体大小/颜色
	for i, v := range source {
		var attr = []string{
			strconv.FormatFloat(float64(v.Offset)/1000, 'f', 2, 64),
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
		ChatID:         strconv.FormatInt(c.seasonId, 10) + "_" + strconv.FormatInt(c.epId, 10),
		Mission:        0,
		MaxLimit:       2000,
		Source:         "k-v",
		SourceProvider: danmaku.Bilibili,
		DataSize:       len(source),
		Danmaku:        data,
	}

	return &xml, nil
}
