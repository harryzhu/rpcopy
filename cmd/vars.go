package cmd

import (
	"errors"
	"io/fs"
	"regexp"
)

const (
	SEP string = "------------------------------------------------------------"
)

var (
	chunkSize int = 64 << 20
	Host      string
	Port      string
)

var (
	timeGetStart   int64                  = GetNowUnix()
	timeGetStop    int64                  = 0
	timeDuration   int64                  = 0
	totalWriteSize int64                  = 0
	totalSpeed     int64                  = 0
	totalNum       int                    = 0
	dirList        map[string]fs.FileInfo = make(map[string]fs.FileInfo, 2048)
	symList        map[string]string      = make(map[string]string, 256)
)

var numStatistics map[string]int

var fextMatch *regexp.Regexp

var (
	ErrNotSymLink error = errors.New("invalid symlink")
)
