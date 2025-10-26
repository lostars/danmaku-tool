## A new danmaku scraper developed by Go

### Milestone

#### Phase 1: offering a danmaku scraper through CLI
- [x] **bilibili** basically usable
- [ ] **iqiyi**
- [x] **tencent** not fully tested
- [ ] ...

#### Phase 2: offering a web server and APIs

#### Phase 3: supporting DanDanPlay API
- [x] `/match`
- [x] `/comment/{id}`

### Installation

Run as CLI:

```
git clone https://github.com/lostars/danmaku-tool.git
cd danmaku-tool
make build
```

Run as docker:
```
docker pull ghcr.io/lostars/danmaku-tool:latest
```

### Configuration

default config file in `~/.config/danmaku-tool/config.yaml` or
same location as the executable file.

You can check the full config [here](configs/config.example.yaml).

### Usage

#### CLI

`danmaku -h` for details.

You can run `danmaku completion` to generate autocompletion for your shell before start.

scrape danmaku:
```
danmaku scrape <id> --platform=bilibili
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

`danmaku server -c /path/to/your/config.yaml -p [port]` to start a web server.
Or u can start as a docker from above.

```docker
docker run -p 8089:8089 --name danmaku \
-v /path/to/your/config.yaml:/app/config.yaml \
ghcr.io/lostars/danmaku-tool:latest
```

### Web API

Only support DandanPlay `/match` and `/comment/{id}` API currently which is enough to most scenarios.


### Reference

* [dandanplay-api](https://api.dandanplay.net/swagger/index.html#/)
* [bilibili-API-collect](https://github.com/SocialSisterYi/bilibili-API-collect)
* [misaka_danmu_server](https://github.com/l429609201/misaka_danmu_server)