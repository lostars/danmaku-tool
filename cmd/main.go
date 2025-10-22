package main

import (
	"danmu-tool/internal/cmd"
	"danmu-tool/internal/config"
	_ "danmu-tool/internal/importer"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var Version = ""
var showVersion = false
var rootCmd = &cobra.Command{
	Use:   "danmaku",
	Short: "danmaku-tool is a danmaku scraper tool",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if config.RunServer {
			// run web server
			os.Exit(0)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(Version)
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&config.RunServer, "server", "s", false, "Start the application as a Web Server")
	rootCmd.PersistentFlags().BoolVarP(&config.Debug, "debug", "d", false, "Enable debug mod")
	rootCmd.PersistentFlags().IntVarP(&config.Port, "port", "p", config.Port, "Port to run the Web Server on (e.g., -p 9000)")
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "show danmaku tool version")

	rootCmd.AddCommand(cmd.ScraperCmd())
}
