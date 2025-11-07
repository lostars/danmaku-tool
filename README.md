## A new danmaku scraper developed by Go

You can run as CLI to scrape danmaku locally and save as xml file.
Or run as a web server to provide DanDanPlay API and enjoy with your compatible player.

Matching may not so precisely and waiting for optimization.
But basically usable.

My goal is **Building a lightweight and stateless(real-time) danmaku server.**

### Milestone

#### Phase 1: offering a danmaku scraper through CLI

The following 4 platforms are not fully tested.
No special season matching support by now.

Get a better match with Emby API enabled.

- [ ] complete danmaku scrage CLI
  - [ ] scrape by BV id of Bilibili
  - [x] save as ASS file
  - [ ] scrape by album of iqiyi
  - [ ] scrape by show of youku
- [x] **bilibili** scrape and DanDan API match 
- [x] **iqiyi** scrape and DanDan API match
- [x] **youku** scrape and DanDan API match
- [x] **tencent** scrape and DanDan API match
- [ ] other platforms...

#### Phase 2: supporting DanDanPlay API
- [x] `/match`
- [x] `/comment/{id}`
- [ ] media search

#### Phase 3: offering management APIs and web UI

Priority is low.

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

* youku video url looks like: https://v.youku.com/v_show/id_XMTA3MDAzODEy.html?s=cc07361a962411de83b1
    id_xxxx xxxx is vid. s=xxxx xxxx is showId


* iqiyi video url looks like: https://www.iqiyi.com/v_19rrk2gwkw.html v_xxx xxx is tvId; https://www.iqiyi.com/a_19rrk2hct9.html a_xxx xxx is albumId


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