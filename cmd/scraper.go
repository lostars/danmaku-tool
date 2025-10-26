package cmd

import (
	"danmu-tool/cmd/flags"
	"danmu-tool/internal/danmaku"
	"danmu-tool/internal/utils"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func scraperCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scrape <id>",
		Short: "scrape danmaku from id",
	}

	platform := flags.FProperty[string]{Flag: "platform", Register: &flags.PlatformCompletion{}, Options: danmaku.ManagerOfDanmaku.GetPlatforms()}
	cmd.Flags().StringVar(&platform.Value, platform.Flag, "", `danmaku platform: 
`+strings.Join(platform.Options, "\n"))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		Init()
		id := args[0]
		if id == "" {
			return errors.New("id is empty")
		}

		var p = danmaku.ManagerOfDanmaku.Scrapers[platform.Value]
		if p == nil {
			return errors.New(fmt.Sprintf("unsupported platform: %s", platform.Value))
		}
		logger := utils.GetComponentLogger("scrape-cmd")
		start := time.Now()
		err := p.Scrape(id)
		logger.Debug("scrape cmd done", "cost_ms", time.Since(start).Milliseconds())
		if err != nil {
			return err
		}

		return nil
	}

	return cmd
}

func init() {
	rootCmd.AddCommand(scraperCmd())
}
