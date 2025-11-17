package cmd

import (
	"danmaku-tool/cmd/flags"
	"danmaku-tool/internal/danmaku"
	"danmaku-tool/internal/utils"
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

	platform := flags.FProperty[string]{Flag: "platform", Register: &flags.PlatformCompletion{}, Options: danmaku.Platforms()}
	cmd.Flags().StringVar(&platform.Value, platform.Flag, "", `danmaku platform: 
`+strings.Join(platform.Options, "\n"))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		Init()
		id := args[0]
		if id == "" {
			return fmt.Errorf("id is empty")
		}

		scraper := danmaku.GetScraper(platform.Value)
		if scraper == nil {
			return fmt.Errorf("unsupported platform: %s", platform.Value)
		}
		start := time.Now()
		if err := scraper.Scrape(id); err != nil {
			utils.ErrorLog(scrapeCmdC, err.Error())
			return nil
		}
		utils.InfoLog(scrapeCmdC, "scrape cmd done", "cost_ms", time.Since(start).Milliseconds())

		return nil
	}

	return cmd
}

const scrapeCmdC = "scrape_cmd"

func init() {
	rootCmd.AddCommand(scraperCmd())
}
