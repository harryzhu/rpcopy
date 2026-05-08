package cmd

import (
	"errors"
	"io/fs"
	"regexp"
)

const (
	MaxBoltSize int64 = 1 << 30
	// < 8 MB :use boltdb
	// >= 8 MB use rawfile
	BoltSplitSize  int64  = 8 << 20
	SEP            string = "------------------------------------------------------------"
	MaxMessageSize int    = 2 << 30
)

var (
	chunkSize int = 128 << 20
	Host      string
	Port      string
)

var (
	timeGetStart   int64                  = GetNowUnix()
	timeGetStop    int64                  = 0
	timeDuration   int64                  = 0
	totalWriteSize int64                  = 0
	totalSpeed     int64                  = 0
	totalNum       int32                  = 0
	chanNum        int32                  = 0
	dirList        map[string]fs.FileInfo = make(map[string]fs.FileInfo, 2048)
	symList        map[string]string      = make(map[string]string, 256)
)

var numStatistics map[string]int

var fextMatch *regexp.Regexp

var (
	ErrNotSymLink error = errors.New("invalid symlink")
)
