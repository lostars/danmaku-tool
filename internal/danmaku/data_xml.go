package danmaku

import (
	"danmu-tool/internal/utils"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
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
		return DataPersistError(XMLPersistType, "parser is nil")
	}

	if e := checkPersistPath(fullPath, filename); e != nil {
		return DataPersistError(XMLPersistType, fmt.Sprintf("%s", e.Error()))
	}

	var data, err = parser.Parse()
	if err != nil {
		return DataPersistError(XMLPersistType, fmt.Sprintf("parse data err: %v", err.Error()))
	}
	var xmlData []byte
	if x.Indent {
		xmlData, err = xml.MarshalIndent(data, "", "    ")
	} else {
		xmlData, err = xml.Marshal(data)
	}
	if err != nil {
		return DataPersistError(XMLPersistType, fmt.Sprintf("marshal error: %v", err))
	}

	// 注意：xml.Marshal 不会自动添加声明头，需要手动添加。
	finalXml := []byte(xml.Header)
	finalXml = append(finalXml, xmlData...)
	writeFile := filepath.Join(fullPath, filename+".xml")
	err = os.WriteFile(writeFile, finalXml, 0644)
	if err != nil {
		return DataPersistError(XMLPersistType, fmt.Sprintf("%s write fail: %v", writeFile, err))
	}

	utils.GetComponentLogger(XMLPersistType).Info("file save success", "file", writeFile)
	return nil
}
