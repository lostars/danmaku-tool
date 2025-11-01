package danmaku

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/utils"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type DataXML struct {
	// root element
	XMLName xml.Name `xml:"i"`
	// metadata
	ChatServer     string           `xml:"chatserver"`
	ChatID         string           `xml:"chatid"`
	Mission        int              `xml:"mission"`
	MaxLimit       int              `xml:"maxlimit"`
	Source         string           `xml:"source"`
	SourceProvider string           `xml:"sourceprovider"`
	DataSize       int              `xml:"datasize"`
	Danmaku        []DataXMLDanmaku `xml:"d"`
}

type DataXMLDanmaku struct {
	// p属性
	Attributes string `xml:"p,attr"`
	Content    string `xml:",chardata"`
}

type DataXMLPersist struct {
	// 缩进
	Indent bool
}

var xmlPersist = DataXMLPersist{Indent: true}

func (x *DataXMLPersist) WriteToFile(data DataXML, fullPath, filename string) error {
	if e := checkPersistPath(fullPath, filename); e != nil {
		return e
	}

	var xmlData []byte
	var err error
	if x.Indent {
		xmlData, err = xml.MarshalIndent(data, "", "    ")
	} else {
		xmlData, err = xml.Marshal(data)
	}
	if err != nil {
		return err
	}

	// 注意：xml.Marshal 不会自动添加声明头，需要手动添加。
	finalXml := []byte(xml.Header)
	finalXml = append(finalXml, xmlData...)
	writeFile := filepath.Join(fullPath, filename+".xml")
	err = os.WriteFile(writeFile, finalXml, 0644)
	if err != nil {
		return err
	}

	utils.GetComponentLogger(XMLSerializer).Info("file save success", "file", writeFile)
	return nil
}

func NormalConvert(source []*StandardDanmaku, platform string, durationInMills int64) []DataXMLDanmaku {
	mergedMills := config.GetConfig().GetPlatformConfig(platform).MergeDanmakuInMills
	if mergedMills > 0 {
		source = MergeDanmaku(source, mergedMills, durationInMills)
	}

	var data = make([]DataXMLDanmaku, 0, len(source))
	// <d p="2.603,1,25,16777215,[tencent]">看看 X2</d>
	// 第几秒/弹幕类型/字体大小/颜色
	for _, v := range source {
		fontSize := "25"
		if v.FontSize > 0 {
			fontSize = strconv.FormatInt(int64(v.FontSize), 10)
		}
		var attr = []string{
			strconv.FormatFloat(float64(v.OffsetMills)/1000, 'f', 2, 64),
			strconv.FormatInt(int64(v.Mode), 10),
			fontSize,
			strconv.FormatInt(int64(v.Color), 10),
			fmt.Sprintf("[%s]", platform),
		}
		d := DataXMLDanmaku{
			Attributes: strings.Join(attr, ","),
			Content:    v.Content,
		}
		data = append(data, d)
	}
	return data
}

func WriteFile(platform Platform, data *SerializerData, savePath, filename string) error {
	logger := utils.GetComponentLogger("serializer")
	serializers := adapter.serializers[string(platform)]
	if serializers == nil {
		logger.Info(fmt.Sprintf("%s no serializer configured", platform))
		return nil
	}

	for _, serializer := range serializers {
		d, err := serializer.Serialize(data)
		if err != nil {
			logger.Info(fmt.Sprintf("%s serialize error: %s", platform, err.Error()))
			continue
		}
		switch t := d.(type) {
		case DataXML:
			err = xmlPersist.WriteToFile(t, savePath, filename)
			if err != nil {
				logger.Error(fmt.Sprintf("xml wirte to file %s error: %s", filepath.Join(savePath, filename), err.Error()))
			}
		}
	}
	return nil
}
