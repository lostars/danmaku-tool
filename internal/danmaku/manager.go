package danmaku

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
)

type Media struct {
}

type Platform interface {
	Platform() string
	Scrape(id interface{}) error
}

type MediaSearcher interface {
	Search(keyword string) ([]Media, error)
	Id(keyword string) ([]interface{}, error)
	Searcher() string
}

type DataPersist interface {
	WriteToFile() error
	Type() string
}

var debugger sync.Map
var dataDebugger sync.Map

func DataDebugger(s DataPersist) *log.Logger {
	var prefix = s.Type()
	v, ok := dataDebugger.Load(prefix)
	if ok {
		logger, err := v.(log.Logger)
		if err {
			return &logger
		}
	}
	// TODO config & 并发 test
	logger := log.New(os.Stdout, fmt.Sprintf("[%s] ", prefix), 0)
	dataDebugger.Store(prefix, logger)
	return logger
}

func NewDataError(d DataPersist, text string) error {
	return errors.New(fmt.Sprintf("[%s]: %s", d.Type(), text))
}

func Debugger(p Platform) *log.Logger {
	var prefix = p.Platform()
	v, ok := debugger.Load(prefix)
	if ok {
		logger, err := v.(log.Logger)
		if err {
			return &logger
		}
	}
	// TODO config & 并发 test
	logger := log.New(os.Stdout, fmt.Sprintf("[%s] ", prefix), 0)
	debugger.Store(prefix, logger)
	return logger
}

func NewError(p Platform, text string) error {
	return errors.New(fmt.Sprintf("[%s] %s", p.Platform(), text))
}

type Manager struct {
	Platforms map[string]Platform
	Searchers map[string]MediaSearcher
}

var ManagerOfDanmaku = &Manager{
	Platforms: map[string]Platform{},
	Searchers: map[string]MediaSearcher{},
}

func (m *Manager) GetPlatforms() []string {
	var result []string
	for _, v := range m.Platforms {
		result = append(result, v.Platform())
	}
	return result
}

func RegisterPlatform(p Platform) error {
	e := ManagerOfDanmaku.Platforms[p.Platform()]
	if e != nil {
		return errors.New(fmt.Sprintf("%s registered", p.Platform()))
	}
	ManagerOfDanmaku.Platforms[p.Platform()] = p
	return nil
}
