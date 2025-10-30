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

type DataXMLParser interface {
	Parse() (*DataXML, error)
}

type DataXMLPersist struct {
	// 缩进
	Indent bool
}

func (x *DataXMLPersist) WriteToFile(parser DataXMLParser, fullPath, filename string) error {
	if parser == nil {
		return fmt.Errorf("%s parser is nil", XMLPersistType)
	}

	if e := checkPersistPath(fullPath, filename); e != nil {
		return e
	}

	var data, err = parser.Parse()
	if err != nil {
		return err
	}
	var xmlData []byte
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

	utils.GetComponentLogger(XMLPersistType).Info("file save success", "file", writeFile)
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
		var attr = []string{
			strconv.FormatFloat(float64(v.OffsetMills)/1000, 'f', 2, 64),
			strconv.FormatInt(int64(v.Mode), 10),
			"25", // 固定字号
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
