package cmd

import (
	"danmu-tool/internal/danmaku"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func ScraperCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "d <id>",
		Short: "scrap danmaku from id",
	}

	platform := FlagsProperty[string]{Flag: "platform", Register: &PlatformCompletion{}, Options: danmaku.ManagerOfDanmaku.GetPlatforms()}
	cmd.Flags().StringVar(&platform.Value, platform.Flag, "", `danmaku platform: 
`+strings.Join(platform.Options, "\n"))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		id := args[0]
		if id == "" {
			return errors.New("id is empty")
		}

		var p = danmaku.ManagerOfDanmaku.Platforms[platform.Value]
		if p == nil {
			return errors.New(fmt.Sprintf("unsupported platform: %s", platform.Value))
		}
		err := p.Scrape(id)
		if err != nil {
			return err
		}

		return nil
	}

	return cmd
}
