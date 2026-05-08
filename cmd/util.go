package cmd

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/zeebo/xxh3"
)

func bootstrap() error {

	numStatistics = make(map[string]int)
	numStatistics["skip_dot_file"] = 0
	numStatistics["skip_file_ext"] = 0
	numStatistics["skip_size_min"] = 0
	numStatistics["skip_size_max"] = 0
	numStatistics["skip_age_min"] = 0
	numStatistics["skip_age_max"] = 0
	numStatistics["skip_exclude_dir"] = 0
	numStatistics["skip_exists"] = 0
	//
	numStatistics["symbol_link"] = 0
	//

	fextMatch = regexp.MustCompile("(?i)" + FileExt)

	return nil
}

func FormatPrint(ftype string, key string, args ...any) error {
	var s []string
	for _, arg := range args {
		s = append(s, fmt.Sprintf("%v", arg))
	}
	var f0 string
	switch {
	case ftype == "short":
		f0 = "%12s: %-20v\n"
	case ftype == "wide":
		f0 = "%25s: %-20v\n"
	default:
		f0 = "%12s: %-20v\n"

	}
	fmt.Printf(f0, key, strings.Join(s, " "))
	return nil
}

func PrintProgress() error {
	flag := int32(0)
	for {
		flag = atomic.LoadInt32(&progressFlag)
		if flag == 3 {
			break
		}
		time.Sleep(2 * time.Second)
		curTotalNum := atomic.LoadInt32(&totalNum)
		curTotalWriteSize := atomic.LoadInt64(&totalWriteSize)
		if IsDebug == false {
			PrintSpinner2(strings.Join([]string{Int32Str(curTotalNum), " => "}, ""),
				strings.Join([]string{Int64Str(curTotalWriteSize >> 20), " MB"}, ""))
		}
	}

	return nil
}

func argsFinfoValidate() error {
	if MinAge != "" {
		minAge64 = TimeStr2Unix(MinAge)

	}

	if MaxAge != "" {
		maxAge64 = TimeStr2Unix(MaxAge)
	}

	if minAge64 > 0 && maxAge64 > 0 && minAge64 > maxAge64 {
		PrintError("argsFinfoValidate", NewError("--min-age= cannot be greater than --max-age= "))
		ExitWithNum(0)
	}

	if MinSizeMB >= 0 {
		MinSize = MinSizeMB << 20
	}

	if MaxSizeMB >= 0 {
		MaxSize = MaxSizeMB << 20
	}

	if MinSize > -1 && MaxSize > -1 && MinSize > MaxSize {
		PrintError("argsFinfoValidate", NewError("--min-size= cannot be greater than --max-size= "))
		ExitWithNum(0)
	}

	if MinSize < -1 || MaxSize < -1 {
		PrintError("argsFinfoValidate", NewError("--min-size= or --max-size= should be greater than 0 "))
		ExitWithNum(0)
	}

	return nil
}

func argsValidate() error {
	if SourceDir != "" {
		FormatPrint("short", "SourceDir", SourceDir)
	}
	if TargetDir != "" {
		FormatPrint("short", "TargetDir", TargetDir)
	}
	fmt.Println(SEP)
	//
	if TargetDir == "/" {
		PrintError("rpcopy", NewError("--target-dir= cannot be \"/\" for safty"))
		ExitWithNum(0)
	}
	if IsZstdSend {
		chunkSize = 8 << 20
	}

	argsFinfoValidate()

	//
	if FileExt != "" {
		FormatPrint("wide", "file-extenion", FileExt)
	} else {
		FormatPrint("wide", "file-extenion", "*")
	}

	FormatPrint("wide", "last-update-time: min", minAge64)
	FormatPrint("wide", "last-update-time: max", maxAge64)
	if MinSize != -1 {
		FormatPrint("wide", "file-size: min", MinSize)
	}

	if MaxSize != -1 {
		FormatPrint("wide", "file-size: max", MaxSize)
	}

	FormatPrint("wide", "ignore-dot-files", IsIgnoreDotFile)
	FormatPrint("wide", "ignore-empty-folder", IsIgnoreEmptyFolder)
	FormatPrint("wide", "overwrite-existing-files", IsOverwrite)
	FormatPrint("wide", "follow-symlink", IsFollowSymlink)
	FormatPrint("wide", "zstd", IsZstdSend)
	FormatPrint("wide", "chunk", chunkSize>>20, "MB")
	FormatPrint("wide", "host", Host)
	FormatPrint("wide", "port", Port)
	FormatPrint("wide", "Time", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println(SEP)
	return nil
}

func isSymlink(src string) bool {
	linfo, err := os.Lstat(src)
	if err != nil {
		PrintError("getSymlink", err)
		return false
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		return true
	}
	return false
}

func isPathValid(p string) bool {
	//ban := []string{":", "\\",  "\"", "*", "?", "<", ">", "|"}
	ban := `:\\*\">?<|`
	if strings.ContainsAny(p, ban) {
		return false
	}
	return true
}

func getSymlink(src string) string {
	linfo, err := os.Lstat(src)
	if err != nil {
		PrintError("getSymlink", err)
		return ""
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		srcLinkTarget, err := os.Readlink(src)
		if err != nil {
			PrintError("getSymlink", err)
			return ""
		}
		return srcLinkTarget
	}
	return ""
}

func MakeDirs(dpath string) error {
	dpath = ToUnixSlash(dpath)
	_, err := os.Stat(dpath)
	if err != nil {
		DebugInfo("MakeDirs", dpath)
		err = os.MkdirAll(dpath, os.ModePerm)
		PrintError("MakeDirs:MkdirAll", err)
		return err
	}
	return nil
}

func MakeSymlink(srcFile string, dstLink string) error {
	srcFile = ToUnixSlash(srcFile)
	dstLink = ToUnixSlash(dstLink)

	_, err := os.Lstat(dstLink)
	if err != nil {
		err := os.Symlink(srcFile, dstLink)
		if err != nil {
			PrintError("MakeSymlink", err)
			return err
		}
	}

	return nil
}

func Map2Byte(m map[string]int64) []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(&m)
	PrintError("Map2Byte", err)
	return buf.Bytes()
}

func Byte2Map(b []byte, m map[string]int64) (map[string]int64, error) {
	buf := bytes.NewBuffer(b)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(&m)
	if err != nil {
		PrintError("Byte2Map: gob.NewDecoder", err)
		return m, err
	}
	return m, nil
}

func Int2Str(n int) string {
	return strconv.Itoa(n)
}

func Int32Str(n int32) string {
	return fmt.Sprintf("%v", n)
}

func Int64Int(n int64) int {
	n10, err := strconv.Atoi(strconv.FormatInt(n, 10))
	if err != nil {
		PrintError("Int64Int", err)
		return 0
	}
	return n10
}

func Int64Str(n int64) string {
	return fmt.Sprintf("%v", n)
}

func GetNowTime() time.Time {
	return time.Now()
}

func GetNowUnix() int64 {
	return time.Now().UTC().Unix()
}

func GetNowUnixMilli() int64 {
	return time.Now().UTC().UnixMilli()
}

func GetNowTimeStr(f string) string {
	switch f {
	case "Ymd":
		return time.Now().Format("20060102")
	case "H":
		return time.Now().Format("15")
	case "His":
		return time.Now().Format("150405")
	default:
		return time.Now().Format("20060102150405")
	}

}

func ToUnixSlash(s string) string {
	// for windows
	return strings.ReplaceAll(s, "\\", "/")
}

func TimeStr2Unix(s string) int64 {
	layout := "2006-01-02,15:04:05"
	var parsedTime time.Time
	var err error

	parsedTime, err = time.ParseInLocation(layout, s, time.Local)

	if err != nil {
		PrintError("TimeStr2Unix", err)
		os.Exit(0)
	}

	return parsedTime.Unix()
}

func ExitWithNum(n int) {
	os.Exit(n)
}

func FileExists(fpath string) bool {
	_, err := os.Stat(fpath)
	if err != nil {
		return false
	}
	return true
}

func ZstdBytes(rawin []byte) []byte {
	enc, _ := zstd.NewWriter(nil)
	return enc.EncodeAll(rawin, nil)
}

func UnZstdBytes(zin []byte) (out []byte, err error) {
	dec, _ := zstd.NewReader(nil)
	out, err = dec.DecodeAll(zin, nil)
	if err != nil {
		PrintError("UnZstdBytes:DecodeAll", err)
		return nil, err
	}
	return out, nil
}

func hashFile(fpath string) string {
	var hasher hash.Hash
	hasher = xxh3.New()

	fh, err := os.Open(fpath)
	if err != nil {
		PrintError("HashFile", err)
	}

	r := bufio.NewReader(fh)

	var buf []byte = make([]byte, 8192)
	for {
		n, err := r.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			PrintError("HashFile", err)
		}
		hasher.Write(buf[:n])
	}

	fh.Close()
	return hex.EncodeToString(hasher.Sum(nil))
}

func hashString(b []byte) string {
	var hasher hash.Hash
	hasher = xxh3.New()
	hasher.Write(b)

	return hex.EncodeToString(hasher.Sum(nil))
}

func isCopyNeeded(fpath string, finfo fs.FileInfo) bool {
	if FileExt != "" {
		if fextMatch.MatchString(filepath.Ext(fpath)) == false {
			numStatistics["skip_file_ext"]++
			return false
		}
	}

	if IsIgnoreDotFile == true {
		if strings.HasPrefix(filepath.Base(fpath), ".") {
			numStatistics["skip_dot_file"]++
			return false
		}
	}

	if finfo == nil {
		return true
	}

	if MinSize != -1 {
		if finfo.Size() < MinSize {
			numStatistics["skip_size_min"]++
			return false
		}
	}

	if MaxSize != -1 {
		if finfo.Size() > MaxSize {
			numStatistics["skip_size_max"]++
			return false
		}
	}

	if MinAge != "" {
		if finfo.ModTime().Unix() < TimeStr2Unix(MinAge) {
			numStatistics["skip_age_min"]++
			return false
		}
	}

	if MaxAge != "" {
		if finfo.ModTime().Unix() > TimeStr2Unix(MaxAge) {
			numStatistics["skip_age_max"]++
			return false
		}
	}

	return true
}

func GetFileList(dirPath string, withFilter bool) (filelist map[string]int64) {
	filelist = make(map[string]int64, 256)
	dirPath = strings.TrimSuffix(ToUnixSlash(dirPath), "/")
	filepath.Walk(dirPath, func(fpath string, finfo fs.FileInfo, err error) error {
		if err != nil {
			PrintError("GetFileList: filepath.Walk", err)
			return err
		}
		fpath = ToUnixSlash(fpath)
		if fpath == "." || fpath == ".." || fpath == "" {
			return nil
		}

		if finfo.IsDir() {
			return nil
		}

		if withFilter {
			if isCopyNeeded(fpath, finfo) == false {
				return nil
			}
		}

		fkey := strings.TrimPrefix(strings.TrimPrefix(fpath, dirPath), "/")
		filelist[fkey] = finfo.Size()

		return nil
	})
	return filelist
}
