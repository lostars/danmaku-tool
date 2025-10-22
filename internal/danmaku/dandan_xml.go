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
	ChatID         int64              `xml:"chatid"`
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

type DanDanXMLGenerator struct {
	Indent   bool
	Parser   DanDanXMLParser
	FullPath string
	Filename string
}

func (x *DanDanXMLGenerator) Type() string {
	return "dandanxml"
}

func (x *DanDanXMLGenerator) WriteToFile() error {
	if x.Parser == nil {
		return NewDataError(x, "transformer is nil")
	}
	if x.FullPath == "" || x.Filename == "" {
		return NewDataError(x, "empty save path or filename")
	}

	// check path
	_, fileStatError := os.Stat(x.FullPath)
	if fileStatError != nil {
		if os.IsNotExist(fileStatError) {
			mkdirError := os.MkdirAll(x.FullPath, os.ModePerm)
			if mkdirError != nil {
				return NewDataError(x, fmt.Sprintf("create path %s error: %s", x.FullPath, mkdirError.Error()))
			}
		} else {
			return NewDataError(x, fmt.Sprintf("create path %s error: %s", x.FullPath, fileStatError.Error()))
		}
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
	writeFile := filepath.Join(x.FullPath, x.Filename+".xml")
	err = os.WriteFile(writeFile, finalXml, 0644)
	if err != nil {
		return NewDataError(x, fmt.Sprintf("%s write fail: %v", writeFile, err))
	}

	DataDebugger(x).Printf("%s wirte success", writeFile)
	return nil
}
