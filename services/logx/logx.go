package logx

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

var (
	mu       sync.Mutex
	minLevel = LevelInfo
)

func SetLevel(l Level) {
	mu.Lock()
	minLevel = l
	mu.Unlock()
}

type levelMeta struct {
	label string
	color *color.Color
}

var metas = map[Level]levelMeta{
	LevelDebug: {"DBG", color.New(color.Faint)},
	LevelInfo:  {"INF", color.New(color.FgCyan)},
	LevelWarn:  {"WRN", color.New(color.FgYellow)},
	LevelError: {"ERR", color.New(color.FgRed)},
	LevelFatal: {"FTL", color.New(color.FgRed, color.Bold)},
}

func output(l Level, format string, args ...any) {
	mu.Lock()
	skip := l < minLevel
	mu.Unlock()
	if skip {
		return
	}
	meta := metas[l]
	ts := time.Now().Format("2006/01/02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(color.Error, "%s %s %s\n", ts, meta.color.Sprint(meta.label), msg)
}

func Debugf(format string, args ...any) { output(LevelDebug, format, args...) }
func Infof(format string, args ...any)  { output(LevelInfo, format, args...) }
func Warnf(format string, args ...any)  { output(LevelWarn, format, args...) }
func Errorf(format string, args ...any) { output(LevelError, format, args...) }

func Fatalf(format string, args ...any) {
	output(LevelFatal, format, args...)
	os.Exit(1)
}
