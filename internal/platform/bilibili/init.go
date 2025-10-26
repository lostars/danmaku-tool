package bilibili

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"danmu-tool/internal/utils"
	"net/http"
	"time"
)

func (c *Client) Init(config *config.DanmakuConfig) error {
	conf := config.Bilibili
	logger = utils.GetPlatformLogger(danmaku.Bilibili)
	c.Cookie = conf.Cookie
	c.MaxWorker = conf.MaxWorker
	c.HttpClient = &http.Client{Timeout: time.Duration(conf.Timeout * 1e9)}
	// 初始化数据存储器
	for _, p := range conf.Persists {
		switch p.Type {
		case danmaku.XMLPersistType:
			c.xmlParser = &danmaku.DataXMLPersist{
				Indent: p.Indent,
			}
		}
	}
	return nil
}

func init() {
	danmaku.Register(&Client{})
}
