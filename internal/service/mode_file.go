package service

import (
	"danmaku-tool/internal/config"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
	"danmaku-tool/internal/web"
	"fmt"
	"time"
)

type fileTimeData struct {
	realTimeData
}

const fileTimeDataC = "file_time_data"

func init() {
	danmaku.Register(&fileTimeData{
		realTimeData{
			ReverseMap: make(map[int64]string),
			ForwardMap: make(map[string]int64),
		},
	})
}

func (c *fileTimeData) ServerInit() error {
	mode := config.GetDandan().Mode
	if mode != file {
		return nil
	}
	if err := c.Load(); err != nil {
		return err
	}
	utils.InfoLog(fileTimeDataC, fmt.Sprintf("[%s] mode enabled", c.Mode()))
	RegisterSource(c)
	danmaku.RegisterFinalizer(c)
	return nil
}

func (c *fileTimeData) GetDanmaku(param CommentParam) (*CommentResult, error) {
	platform, ssId, epId, found := c.decodeGlobalID(param.Id)
	if !found {
		return nil, web.ErrBadReqeustOf("invalid param")
	}

	// load from file
	data := danmaku.DeserializeDanmaku(platform, ssId, epId)
	comment := &CommentResult{
		Count:    int64(len(data)),
		Comments: make([]*Comment, 0, len(data)),
	}
	for _, d := range data {
		comment.Comments = append(comment.Comments, &Comment{
			CID: time.Now().Unix(),
			M:   d.Content,
			P:   d.GenDandanAttribute(),
		})
	}
	return comment, nil
}

func (c *fileTimeData) Mode() Mode {
	return file
}
