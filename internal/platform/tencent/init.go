package tencent

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"danmu-tool/internal/utils"
	"net/http"
	"time"
)

func (c *Client) Init(config *config.DanmakuConfig) error {
	logger = utils.GetPlatformLogger(danmaku.Tencent)
	conf := config.Tencent

	c.Cookie = conf.Cookie
	c.MaxWorker = conf.MaxWorker
	c.HttpClient = &http.Client{Timeout: time.Duration(conf.Timeout * 1e9)}
	c.DataPersists = []danmaku.DataPersist{}

	// 初始化数据存储器
	for _, p := range conf.Persists {
		switch p.Name {
		case danmaku.XMLPersistType:
			persist := danmaku.DataXMLPersist{
				Indent: p.Indent,
				Parser: c,
			}
			c.DataPersists = append(c.DataPersists, &persist)
		}
	}
	return nil
}

func init() {
	danmaku.Register(&Client{})
}
