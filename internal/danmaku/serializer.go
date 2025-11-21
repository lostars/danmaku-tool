package danmaku

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/utils"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

func (x *DataXMLPersist) Deserialize(file string) ([]*StandardDanmaku, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var dataXML DataXML
	if err = xml.Unmarshal(data, &dataXML); err != nil {
		return nil, err
	}

	var result = make([]*StandardDanmaku, 0, len(dataXML.Danmaku))
	for _, d := range dataXML.Danmaku {
		attrs := strings.Split(d.Attributes, ",")
		if len(attrs) < 4 {
			continue
		}
		// 注意xml中offset存储的是秒
		offsetInSeconds, pos, fontSize, color := float64(0), int64(0), int64(0), int64(0)
		if offsetInSeconds, err = strconv.ParseFloat(attrs[0], 64); err != nil {
			continue
		}
		if pos, err = strconv.ParseInt(attrs[1], 10, 64); err != nil {
			continue
		}
		if fontSize, err = strconv.ParseInt(attrs[2], 10, 64); err != nil {
			continue
		}
		if color, err = strconv.ParseInt(attrs[3], 10, 64); err != nil {
			continue
		}
		result = append(result, &StandardDanmaku{
			Content:     d.Content,
			OffsetMills: int64(offsetInSeconds) * 1000,
			Mode:        int(pos),
			FontSize:    int32(fontSize),
			Color:       int(color),
		})
	}

	return result, nil
}

func init() {
	xmlPersist := DataXMLPersist{Indent: true}
	adapter.serializers[xmlPersist.Type()] = &xmlPersist
	assPersist := DataAssPersist{}
	adapter.serializers[assPersist.Type()] = &assPersist
}

func (x *DataXMLPersist) Type() string {
	return XMLSerializer
}

func (x *DataXMLPersist) Serialize(s *SerializerData) error {
	fullPath := s.FullPath
	filename := s.Filename
	if e := s.CheckPersistPath(); e != nil {
		return e
	}

	data := NormalConvert(s)

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

	utils.InfoLog(XMLSerializer, "file save success", "file", writeFile)
	return nil
}

func NormalConvert(s *SerializerData) *DataXML {
	source := s.Data
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
			fmt.Sprintf("[%s]", s.Platform),
		}
		d := DataXMLDanmaku{
			Attributes: strings.Join(attr, ","),
			Content:    v.Content,
		}
		data = append(data, d)
	}

	result := &DataXML{
		ChatServer:     s.SeasonId + "_" + s.EpisodeId,
		ChatID:         s.SeasonId + "_" + s.EpisodeId,
		Mission:        0,
		MaxLimit:       2000,
		Source:         "k-v",
		SourceProvider: string(s.Platform),
		DataSize:       len(source),
		Danmaku:        data,
	}

	return result
}

const serializerC = "serializer"

func WriteFile(data *SerializerData) {
	conf := config.GetPlatformConfig(string(data.Platform))
	if conf == nil {
		return
	}
	// 合并弹幕
	mergedMills := conf.MergeDanmakuInMills
	if mergedMills > 0 {
		data.Data = MergeDanmaku(data.Data, mergedMills, data.DurationInMills)
	}
	for _, s := range conf.Persists {
		serializer := adapter.serializers[s]
		if serializer == nil {
			utils.ErrorLog(serializerC, "serializer not impl", "platform", data.Platform, "serializer", s)
			continue
		}

		if err := serializer.Serialize(data); err != nil {
			utils.ErrorLog(serializerC, err.Error(), "platform", data.Platform, "serializer", serializer.Type())
		}
	}
}

func CheckFile(data *SerializerData) error {
	conf := config.GetPlatformConfig(string(data.Platform))
	if conf == nil {
		return fmt.Errorf("[%s] config not exist", data.Platform)
	}
	for _, s := range conf.Persists {
		if err := data.CheckExistFile(s); err != nil {
			return err
		}
	}
	return nil
}

func (s SerializerData) CheckExistFile(ext string) error {
	// check file status
	full := filepath.Join(s.FullPath, fmt.Sprintf("%s.%s", s.Filename, ext))
	info, err := os.Stat(full)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	conf := config.GetPlatformConfig(string(s.Platform))
	if conf == nil {
		return fmt.Errorf("file [%s] exists", full)
	}
	if !conf.FileExpire(info.ModTime()) {
		return fmt.Errorf("file [%s] exists and not exipre", full)
	}
	return nil
}

func (s SerializerData) CheckPersistPath() error {
	if s.FullPath == "" || s.Filename == "" {
		return fmt.Errorf("empty save path or filename")
	}

	// check path
	_, fileStatError := os.Stat(s.FullPath)
	if fileStatError != nil {
		if os.IsNotExist(fileStatError) {
			if mkdirError := os.MkdirAll(s.FullPath, os.ModePerm); mkdirError != nil {
				return fmt.Errorf("create path %s error: %s", s.FullPath, mkdirError.Error())
			}
		} else {
			return fmt.Errorf("create path %s error: %s", s.FullPath, fileStatError.Error())
		}
	}
	return nil
}

type DataAssPersist struct{}

func (a *DataAssPersist) Deserialize(_ string) ([]*StandardDanmaku, error) {
	return nil, fmt.Errorf("%s deserialize not support", a.Type())
}

func (a *DataAssPersist) Type() string {
	return ASSSerializer
}

func (a *DataAssPersist) Serialize(data *SerializerData) error {
	savePath := data.FullPath
	filename := data.Filename
	if e := data.CheckPersistPath(); e != nil {
		return e
	}
	if data.ResX == 0 || data.ResY == 0 {
		data.ResX = 1920
		data.ResY = 1080
		utils.WarnLog(ASSSerializer, fmt.Sprintf("ass serializer fallback to default resolution, episodeId: %s", data.EpisodeId))
	}

	scriptInfoLines := []string{
		"[Script Info]",
		"; Generated by danmaku-tool",
		"; https://github.com/lostars/danmaku-tool",
		"Title: " + data.EpisodeId,
		"ScriptType: v4.00+",
		"PlayResX: " + strconv.FormatInt(int64(data.ResX), 10),
		"PlayResY: " + strconv.FormatInt(int64(data.ResY), 10),
		"Timer: 10.0000",
		"WrapStyle: 2",
		"ScaledBorderAndShadow: no",
	}
	scriptInfo := strings.Join(scriptInfoLines, "\n")

	fontStyle := "Style: %s,%s,%d,&H66FFFFFF,&H66FFFFFF,&H66000000,&H66000000,0,0,0,0,100,100,0,0,1,1.2,0,5,0,0,0,0"
	font := sysFont[runtime.GOOS]
	if font == "" {
		font = "黑体"
	}
	fontSize := data.ResY / 30
	stylesLines := []string{
		"[V4+ Styles]",
		"Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding",
		fmt.Sprintf(fontStyle, "Small", font, fontSize),
		fmt.Sprintf(fontStyle, "Medium", font, fontSize+15),
		fmt.Sprintf(fontStyle, "Large", font, fontSize+30),
		fmt.Sprintf(fontStyle, "Larger", font, fontSize+45),
		fmt.Sprintf(fontStyle, "ExtraLarge", font, fontSize+60),
	}
	styles := strings.Join(stylesLines, "\n")

	var eventsLines = make([]string, 0, len(data.Data))
	eventsLines = append(eventsLines, "[Events]", "Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text")
	for i, d := range data.Data {
		eventsLines = append(eventsLines, a.buildDialogue(d, data.ResX, data.ResY, i))
	}
	events := strings.Join(eventsLines, "\n")

	writeFile := filepath.Join(savePath, filename+".ass")
	file, e := os.Create(writeFile)
	if e != nil {
		return e
	}
	defer utils.SafeClose(file)
	assStr := strings.Join([]string{scriptInfo, styles, events}, "\n\n")
	if _, err := file.WriteString(assStr); err != nil {
		return err
	}
	utils.InfoLog(ASSSerializer, "file save success", "file", writeFile)
	return nil
}

func (a *DataAssPersist) buildDialogue(data *StandardDanmaku, x, y int, index int) string {
	assDuration := int64(6000)
	//Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
	//Dialogue: 0,0:00:00.00,0:00:06.00,Medium,,0,0,0,,{\move(2206,30,-286,30,0,6000)\c&HFFFFFF&}华夏男儿必须全力争胜！
	dialogue := []string{"Dialogue: 0"}
	// start end
	dialogue = append(dialogue, toASSTime(float64(data.OffsetMills)/1000), toASSTime(float64(data.OffsetMills+assDuration)/1000))
	dialogue = append(dialogue, "Medium", "", "0", "0", "0", "")
	// effect:
	// pos(960,30) move(2206,30,-286,30,0,6000)
	var move string
	lineHeight := 30
	textWidth := len([]rune(data.Content)) * 30
	switch data.Mode {
	case NormalMode:
		// 根据弹幕index均匀分布滚动弹幕
		track := index % (y / lineHeight)
		x1 := x + textWidth
		x2 := -textWidth
		y1 := track*lineHeight + lineHeight/2
		y2 := y1
		move = fmt.Sprintf("move(%d,%d,%d,%d,0,%d)", x1, y1, x2, y2, assDuration)
	case BottomMode:
		move = fmt.Sprintf("pos(%d, %d)", x/2, y-lineHeight/2)
	case TopMode:
		move = fmt.Sprintf("pos(%d, %d)", x/2, lineHeight/2)
	}

	color := ""
	if data.Color > 0 {
		color = fmt.Sprintf("%06X", data.Color)
	} else {
		color = fmt.Sprintf("%06X", WhiteColor)
	}
	effect := "{\\%s\\c&H%s&}"
	dialogue = append(dialogue, fmt.Sprintf(effect, move, color))

	return strings.Join(dialogue, ",") + data.Content
}

func toASSTime(sec float64) string {
	h := int(sec) / 3600
	m := (int(sec) % 3600) / 60
	s := int(sec) % 60
	cs := int((sec - float64(int(sec))) * 100) // 百分之一秒
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}

var sysFont = map[string]string{
	"darwin":  "黑体",
	"linux":   "黑体",
	"windows": "微软雅黑",
}
