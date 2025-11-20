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
	var parse bool
	cmd.Flags().BoolVar(&parse, "parse", false, "parse media id string to number")

	cmd.Run = func(cmd *cobra.Command, args []string) {
		Init()
		id := args[0]
		if id == "" {
			utils.ErrorLog(scrapeCmdC, "id is empty")
			return
		}

		scraper := danmaku.GetScraper(platform.Value)
		if scraper == nil {
			if !danmaku.ValidPlatform(platform.Value) {
				utils.ErrorLog(scrapeCmdC, fmt.Sprintf("unsupported platform: %s", platform.Value))
			}
			return
		}
		if parse {
			if parser, ok := scraper.(danmaku.MediaIdParser); ok {
				fmt.Println(fmt.Sprintf("%s -> %d", id, parser.ParseNumber(id)))
			} else {
				utils.ErrorLog(scrapeCmdC, fmt.Sprintf("[%s] not support", scraper.Platform()))
			}
		} else {
			start := time.Now()
			if err := scraper.Scrape(id); err != nil {
				utils.ErrorLog(scrapeCmdC, err.Error())
				return
			}
			utils.InfoLog(scrapeCmdC, "scrape cmd done", "cost_ms", time.Since(start).Milliseconds())
		}
	}

	return cmd
}

const scrapeCmdC = "scrape_cmd"

func init() {
	rootCmd.AddCommand(scraperCmd())
}
