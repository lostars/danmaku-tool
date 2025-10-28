package tencent

import (
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
)

func (c *client) Init(config *config.DanmakuConfig) error {
	common, err := danmaku.InitPlatformClient(danmaku.Tencent)
	if err != nil {
		return err
	}
	c.common = common
	return nil
}

func init() {
	danmaku.Register(&client{})
}
