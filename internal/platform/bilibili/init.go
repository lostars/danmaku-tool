package bilibili

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"danmu-tool/internal/utils"
	"net/http"
	"time"
)

func (c *client) Init(config *config.DanmakuConfig) error {
	conf := config.GetPlatformConfig(danmaku.Bilibili)
	logger = utils.GetPlatformLogger(danmaku.Bilibili)
	if conf == nil || conf.Name == "" {
		logger.Info("platform is not configured")
		return nil
	}
	if conf.Priority <= 0 {
		logger.Info("platform disabled")
		return nil
	}

	c.Cookie = conf.Cookie
	c.MaxWorker = conf.MaxWorker
	if c.MaxWorker <= 0 {
		c.MaxWorker = 4
	}
	var timeout = conf.Timeout
	if timeout <= 0 {
		timeout = 60
	}
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
	danmaku.Register(&client{})
}
