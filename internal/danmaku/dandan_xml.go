package danmaku

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
)

type DanDanXML struct {
	// root element
	XMLName xml.Name `xml:"i"`
	// metadata
	ChatServer     string             `xml:"chatserver"`
	ChatID         string             `xml:"chatid"`
	Mission        int                `xml:"mission"`
	MaxLimit       int                `xml:"maxlimit"`
	Source         string             `xml:"source"`
	SourceProvider string             `xml:"sourceprovider"`
	DataSize       int                `xml:"datasize"`
	Danmaku        []DanDanXMLDanmaku `xml:"d"`
}

type DanDanXMLDanmaku struct {
	// XML 属性 (通过 ,attr 标签指定)
	Attributes string `xml:"p,attr"` // 映射到 p 属性

	// XML 元素的内容
	Content string `xml:",chardata"` // 映射到 <d> 和 </d> 之间的文本内容
}

type DanDanXMLParser interface {
	Parse() (*DanDanXML, error)
}

type DanDanXMLPersist struct {
	Indent bool
	Parser DanDanXMLParser
}

func (x *DanDanXMLPersist) Type() string {
	return DanDanXMLPersistType
}

const DanDanXMLPersistType = "dandanxml"

func (x *DanDanXMLPersist) WriteToFile(fullPath, filename string) error {
	if x.Parser == nil {
		return NewDataError(x, "parser is nil")
	}

	if e := checkPersistPath(fullPath, filename); e != nil {
		return NewDataError(x, fmt.Sprintf("%s", e.Error()))
	}

	var data, err = x.Parser.Parse()
	if err != nil {
		return NewDataError(x, fmt.Sprintf("parse data err: %v", err.Error()))
	}
	var xmlData []byte
	if x.Indent {
		xmlData, err = xml.MarshalIndent(data, "", "    ")
	} else {
		xmlData, err = xml.Marshal(data)
	}
	if err != nil {
		return NewDataError(x, fmt.Sprintf("marshal error: %v", err))
	}

	// 注意：xml.Marshal 不会自动添加声明头，需要手动添加。
	finalXml := []byte(xml.Header)
	finalXml = append(finalXml, xmlData...)
	writeFile := filepath.Join(fullPath, filename+".xml")
	err = os.WriteFile(writeFile, finalXml, 0644)
	if err != nil {
		return NewDataError(x, fmt.Sprintf("%s write fail: %v", writeFile, err))
	}

	DataDebugger(x).Printf("%s wirte success\n", writeFile)
	return nil
}
