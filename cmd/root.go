package cmd

import (
	"danmu-tool/cmd/flags"
	_ "danmu-tool/internal/platform"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "danmaku",
	Short: "danmaku-tool is a danmaku scraper tool",
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&flags.Debug, "debug", "d", false, "enable debug mode")
	rootCmd.PersistentFlags().StringVarP(&flags.ConfigPath, "config", "c", "", "config path")
}
