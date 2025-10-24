## A new danmaku scraper developed by Go

### Milestone

#### Phase 1: offering a danmaku scraper through CLI
- [ ] bilibili
- [ ] iqiyi
- [ ] tencent
- [ ] ...

#### Phase 2: offering a web server and APIs 

#### Phase 3: supporting DanDanPlay API

### Installation

```
git clone https://github.com/lostars/danmaku-tool.git
cd danmaku-tool
make build
```

### Configuration

default config file in `~/.config/danmaku-tool/config.yaml` or
same location as the executable file.

You can check the full config [here](https://github.com/lostars/danmaku-tool/blob/main/configs/config.example.yaml).

### Usage

#### CLI

`danmaku -h` for details.

You can run `danmaku completion` to generate autocompletion for your shell before start.

scrape danmaku:
```
danmaku d <id> --platform=bilibili
```
**id**

* bilibili support epid(ep269616) or ssid(ss28564) from url:

    https://www.bilibili.com/bangumi/play/ss28564 or https://www.bilibili.com/bangumi/play/ep269616

    `danmaku d ss28564 --platform=bilibili`

    Notice that using ssid will scrape all EP's danmaku and download.

    And using epid only download the corresponding danmaku.
* tencent video support cid/vid from url:
  
    https://v.qq.com/x/cover/znda81ms78okdwd/e00242bvw06.html
    `znda81ms78okdwd` is cid, `e00242bvw06` is vid
    `danmaku d znda81ms78okdwd/e00242bvw06 --platform=tencent`

#### WebServer

### Web API

### Reference

[bilibili-API-collect](https://github.com/SocialSisterYi/bilibili-API-collect)
[misaka_danmu_server](https://github.com/l429609201/misaka_danmu_server)