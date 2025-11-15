## 食用方法

数据源目前支持了 `bilibili` `tencent` `youku` `iqiyi`，后续可能会有其他源的接入。

支持命令行和web server两种工作模式，命令行主要用来抓取弹幕到本地，web server主要用于提供dandan兼容的弹幕API。

### 配置

命令行和web server共用配置文件：[config.example.yaml](configs/config.example.yaml)。

核心需要配置的是 bilibili/tencent 的 cookie 和 Emby API（帮助更加精准匹配，不配置也行）。
其他的参照示例配置文件配置即可。

`platforms` name目前支持上面提到的4种，弹幕保存格式支持 `ass` `xml`。
API的token也是通过配置文件简单实现，根据需要自行配置。

**注意**：

* 弹幕保存路径也会用于存储系统映射数据，命名为 `data.gob.gz`。
* 配置文件 `tokenizer` 部分属于试验性功能，直接使用即可，未来可能会调整。
* 不要设置太高的并发，很容易触发平台限流风控。同时各个平台弹幕分片规则不尽相同，调高了也不一定能提升速度。

最小化配置：
```yaml
# 弹幕保存路径 只作用于CLI模式 web server可以不用配置
save-path: ""
# 复制即可
dandan-mode: "real_time"
# 复制一份即可
tokenizer:
  enable: true
  blacklist:
    - regex: "(刺客)*伍六七.*记忆碎片"
      replacement: "刺客伍六七第五季"
    - regex: "(刺客)*伍六七.*暗影宿命"
      replacement: "刺客伍六七第四季"
    - regex: "(刺客)*伍六七.*玄武国篇"
      replacement: "刺客伍六七第三季"
    - regex: "(刺客)*伍六七.*最强发型师"
      replacement: "刺客伍六七第二季"
    - regex: "(刺客)*伍六七"
      replacement: "刺客伍六七"
    - regex: "仙剑奇侠传\\s*第*\\s*(一|1)\\s*部*"
      replacement: "仙剑奇侠传"
server:
  # token配置
  tokens:
    - "xxx"
    - "aaa"
# emby 配置
emby:
  url: ""
  user: ""
  token: ""
#  目前可选 bilibili tencent youku iqiyi 配置均通用
platforms:
  - name: "bilibili"
  #  优先级 用于控制剧集搜索结果 越小则排的更靠前(int) <0 则禁用该平台
    priority: 10
  #  完整cookie 否则部分接口不出数据 bilibili和tencent需要配置
    cookie: ""
  #  合并多少毫秒内弹幕 可以不配置 作用于命令行本地弹幕保存和弹幕接口返回数据
    merge-danmaku-in-mills: 1000
  #  命令行模式保存弹幕格式 web server模式可不用配置
    persists: ["xml", "ass"]
```

### 作为命令行使用

从Release下载编译好的二进制，执行 `danmaku -h` 即可看到支持的命令。

目前仅支持弹幕抓取子命令，`danmaku scrape -h` 获取可用平台参数配置。

```
scrape danmaku from id

Usage:
  danmaku scrape <id> [flags]

Flags:
  -h, --help              help for scrape
      --platform string   danmaku platform: 
                          bilibili
                          tencent
                          youku
                          iqiyi

Global Flags:
  -c, --config string   config path
  -d, --debug           enable debug mode
```

* bilibili 支持 剧集ID 和 集ID 的抓取，即 ss1234 和 ep1234。
  如果是剧集ID，则会抓取剧集所有的集的弹幕。
* tencent 支持单集和剧集的弹幕抓取 比如：https://v.qq.com/x/cover/mzc00200aaogpgh/r0047gdjpw6.html `r0047gdjpw6` `mzc00200aaogpgh` 就是对应的ID。
* iqiyi 支持单集的弹幕抓取，比如： https://www.iqiyi.com/v_19rrk2gwkw.html `19rrk2gwkw` 就是对应ID。
* youku 支持单集的弹幕抓取，比如：https://v.youku.com/v_show/id_XNjQ5NzI5MTY0MA==.html?s=ecda347687c4441cb2f3 `XNjQ5NzI5MTY0MA==` 就是对应ID。


弹幕文件可选保存为 `xml` `ass`，存储结构如下：
```
path
├── bilibili
│   └── 28747
│       ├── 1231576.ass
│       └── 1231576.xml
├── iqiyi
│	└── 103398001
│	    ├── 103411100.ass
│	    └── 103411100.xml
├── tencent
│	└── mzc00200aaogpgh
│	    ├── r0047gdjpw6.ass
│	    └── r0047gdjpw6.xml
└── youku
    └── ecda347687c4441cb2f3
        ├── XNjQ5NzI5MTY0MA==.ass
        └── XNjQ5NzI5MTY0MA==.xml
```

配置文件中的保存路径仅支持上面 `path` 顶级目录的自定义。

bilibili 是以 ss/ep ID的模式组织文件；
iqiyi 是通过 album/tv ID的模式组织文件，只不过是转换过后的数字ID；
tencent 是以 cid/vid 的模式组织文件；
youku 是以 show/id 的模式组织文件；

注意配置文件中的合并弹幕配置，默认 `-d` 打开 `debug` 模式可以看到相关日志：
```
2025-xx-xx xx:xx:xx DEBUG danmaku size merge start component=manager_util size=11587
2025-xx-xx xx:xx:xx DEBUG danmaku size merge end component=manager_util size=11229 cost_ms=1
```

成功抓取并保存输出：
```
2025-xx-xx xx:xx:xx INFO  file save success component=xml file=path/youku/ecda347687c4441cb2f3/XNjQ5NzI5MTY0MA==.xml
2025-xx-xx xx:xx:xx INFO  file save success component=ass file=path/youku/ecda347687c4441cb2f3/XNjQ5NzI5MTY0MA==.ass
```

### 作为web server使用

服务端模式除了必要的数据映射关系数据，不会保存任何弹幕到服务端，均是通过实时请求获取最新的弹幕。
弹幕数据做了一小时的内存缓存，防止反复的打开同一个视频触发反复从平台拉取弹幕。

目前仅兼容了常见播放器调用的dandan API，即自动匹配和手动搜索弹幕功能。
直接拉取镜像即可，目前支持 `amd64/arm64` 架构。

token在配置文件 `server - tokens`，需进行手动配置。

```docker
docker pull ghcr.io/lostars/danmaku-tool:latest

docker run -p 8089:8089 --name danmaku \
-v /path/to/config.yaml:/app/config.yaml \
ghcr.io/lostars/danmaku-tool:latest
```

镜像工作目录在 `/app`，最好将这个目录映射出去，程序默认读取该目录下的 `config.yaml`，同时也会持久化一些系统数据。


#### 或者从命令行启动服务
```
danmaku server -c /path/to/config.yaml -p 8089
```