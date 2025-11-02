package cmd

import (
	"danmaku-tool/internal/config"
	"fmt"

	"github.com/spf13/cobra"
)

func versionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "show version info",
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		fmt.Println(config.Version)
	}

	return cmd
}

func init() {
	rootCmd.AddCommand(versionCmd())
}
