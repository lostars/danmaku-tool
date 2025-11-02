package cmd

import (
	"danmu-tool/cmd/flags"
	"danmu-tool/internal/config"
	"danmu-tool/internal/danmaku"
	"danmu-tool/internal/service"
	"danmu-tool/internal/utils"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func Init() {
	// init config
	config.Init(flags.ConfigPath, flags.Debug)
	// init logger
	utils.LoggerConf.InitLogger(flags.Debug)
	// initializers
	for _, init := range danmaku.GetInitializers() {
		if i, ok := init.(danmaku.Initializer); ok {
			if err := i.Init(); err != nil {
				_, _ = fmt.Fprintf(os.Stdout, "initialize info: %v\n", err)
			}
		}
	}
}

func InitServer() {
	// 初始化GoJieba
	createTempDictsFromEmbed()
	// 其他初始化
	for _, init := range danmaku.GetInitializers() {
		if i, ok := init.(danmaku.ServerInitializer); ok {
			if err := i.ServerInit(); err != nil {
				_, _ = fmt.Fprintf(os.Stdout, "server initialize info: %v\n", err)
			}
		}
	}
}

func Release() {
	mode := service.GetDandanSourceMode()
	if re, ok := mode.(danmaku.Finalizer); ok {
		err := re.Finalize()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "release source error: %v\n", err)
		}
	}
	// 清理GoJieba字典临时文件夹
	if err := os.RemoveAll(jiebaTempDir); err != nil {
		fmt.Printf("jieba temp dir cleanup fail %s: %v", jiebaTempDir, err)
	} else {
		fmt.Println("jieba temp dir cleanup done")
	}
}

var dicts = []string{"jieba.dict.utf8", "hmm_model.utf8", "user.dict.utf8", "idf.utf8", "stop_words.utf8"}

const tempDirPrefix = "jieba_temp_dict_"

var jiebaTempDir = ""

func createTempDictsFromEmbed() {
	tempDir, err := os.MkdirTemp("", tempDirPrefix)
	if err != nil {
		panic(err)
	}

	for _, dict := range dicts {
		// 读取时候需要使用定义 embed.FS 的前缀
		filename := filepath.Join("dist", "dict", dict)
		content, err := fs.ReadFile(flags.JiebaDict, filename)
		if err != nil {
			panic(err)
		}
		// 写入物理文件系统 使用的实际物理路径
		destPath := filepath.Join(tempDir, dict)
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			panic(err)
		}
		config.JiebaDictTempDirs = append(config.JiebaDictTempDirs, destPath)
	}
	jiebaTempDir = tempDir
}
