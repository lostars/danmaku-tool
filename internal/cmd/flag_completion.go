package cmd

import (
	"danmu-tool/internal/danmaku"
	"strings"

	"github.com/spf13/cobra"
)

type FlagsProperty[T any] struct {
	Value    T
	Flag     string
	Register FlagsPropertyRegister
	Options  []string
}

type FlagsPropertyRegister interface {
	complete(toComplete string) []string
}

func (f *FlagsProperty[T]) RegisterCompletion(cmd *cobra.Command) {
	if f.Options != nil && len(f.Options) > 0 {
		_ = cmd.RegisterFlagCompletionFunc(f.Flag, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return f.Options, cobra.ShellCompDirectiveNoFileComp
		})
		return
	}
	if f.Register != nil && f.Flag != "" {
		_ = cmd.RegisterFlagCompletionFunc(f.Flag, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return f.Register.complete(toComplete), cobra.ShellCompDirectiveNoFileComp
		})
		return
	}
}

type PlatformCompletion struct{}

func (p *PlatformCompletion) complete(toComplete string) []string {
	var platforms = danmaku.ManagerOfDanmaku.GetPlatforms()
	var result []string
	for _, v := range platforms {
		if strings.HasPrefix(v, toComplete) {
			result = append(result, v)
		}
	}
	return result
}
