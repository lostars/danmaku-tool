package main

import (
	"danmu-tool/cmd"
	"danmu-tool/cmd/flags"
	"embed"
)

// JieBaDict 只能从当前文件夹往下读取
//
//go:embed dist/dict
var JieBaDict embed.FS

func main() {
	flags.JiebaDict = &JieBaDict
	cmd.Execute()
}
