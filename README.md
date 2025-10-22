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
danmaku d --platform=bilibili
```

#### WebServer

### Web API