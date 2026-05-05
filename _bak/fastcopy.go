package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

type CopyElement struct {
	Fsrc     string
	Fdst     string
	Finfo    os.FileInfo
	CopyMode int
	// 4k align
	z struct{}
}

func updateTotalSpeed() {
	timeGetStop = GetNowUnix()
	timeDuration = timeGetStop - timeGetStart
	if timeDuration > 0 {
		totalSpeed = totalWriteSize / timeDuration
	}

	if IsWithMemStats {
		runtime.ReadMemStats(&memStats)
		memString = fmt.Sprintf("MEM: %vMB,%vMB,NumGC: %v", memStats.Alloc/uMB, memStats.Sys/uMB, memStats.NumGC)
	}
}

func file2CopyElement(srcPath string, dstPath string, finfo os.FileInfo, copyMode int) (ele CopyElement, err error) {
	ele.Fsrc = ToUnixSlash(srcPath)
	ele.Fdst = ToUnixSlash(dstPath)
	ele.Finfo = finfo
	ele.CopyMode = copyMode

	return ele, nil
}

func getChanFileToDisk(ele CopyElement) error {
	if ele.Finfo.IsDir() {
		return nil
	}

	fsrc := ele.Fsrc
	fdst := ele.Fdst
	finfo := ele.Finfo

	DebugInfo("GetChanFileToDisk:", "fsrc = ", fsrc, ", fdst = ", fdst)

	_, err := copyFile(fsrc, fdst, finfo)

	PrintError("GetChanFileToDisk", err)
	return err
}

func fastCopy() error {
	wg := sync.WaitGroup{}
	wg.Add(3)

	go func() error {
		defer wg.Done()
		for {
			if isChanFileRWDone == true {
				break
			}

			if totalNum > 99 || totalNum%100 == 0 {
				updateTotalSpeed()
				fmt.Printf(" %s %6d, %8d, %10dMB, %6dMB/s,  %s\r", ":::", len(chanFile), totalNum, totalWriteSize>>20, totalSpeed>>20, memString)
			}
		}
		return nil
	}()

	//chanFile
	go func() error {
		defer wg.Done()
		taskChanFile()
		updateTotalSpeed()
		return nil
	}()

	go func() error {
		defer wg.Done()

		if IsSerial {
			taskWalkSerial()
		} else {
			taskWalkAsync()
		}

		updateTotalSpeed()
		//
		chanFile <- copyAllDone

		return nil
	}()

	wg.Wait()

	close(chanFile)

	printCopyResult()

	return nil
}

func printCopyResult() error {
	fmt.Printf("\n%s\n", SEP)
	updateTotalSpeed()
	var allIgnoredFiles int
	for k, v := range numStatistics {
		if strings.HasPrefix(k, "skip_") {
			fmt.Printf("** Ignored: %20s: %10v\n", k, v)
			allIgnoredFiles += v
		}
	}

	fmt.Println("")

	for k, v := range numStatistics {
		if strings.HasPrefix(k, "skip_") == false {
			fmt.Printf("** Copied: %21s: %10v", k, v)
		}
	}

	fmt.Printf("\n%s\n", SEP)
	fmt.Printf("** Total: %d, Copied: %d, Write: %d MB, Speed: %d MB/s **\n", totalNum, (totalNum - allIgnoredFiles), totalWriteSize>>20, totalSpeed>>20)

	return nil
}

func purgeTargetDir() error {
	SourceDir = ToUnixSlash(SourceDir)
	TargetDir = ToUnixSlash(TargetDir)
	filepath.WalkDir(TargetDir, func(dstPath string, dirInfo fs.DirEntry, err error) error {
		if err != nil {
			PrintError("purgeTargetDir: walkdir", err)
			return err
		}

		dstPath = ToUnixSlash(dstPath)

		srcPath := strings.Replace(dstPath, strings.TrimRight(TargetDir, "/"), strings.TrimRight(SourceDir, "/"), 1)
		if FileExists(srcPath) == false {
			err = os.Remove(dstPath)
			PrintError("purgeTargetDir:os.Remove", err)
		}

		return nil
	})
	return nil
}

func updateTargetDir() error {
	if len(dirList) > 0 {
		var err error
		t1 := GetNowUnix()
		var srcInfo fs.FileInfo
		for dstPath, srcDirInfo := range dirList {
			srcInfo, _ = srcDirInfo.Info()
			err = os.Chtimes(dstPath, srcInfo.ModTime(), srcInfo.ModTime())
			PrintError("updateTargetDir: os.Chtimes", err)

			err = os.Chmod(dstPath, srcInfo.Mode())
			PrintError("updateTargetDir: os.Chmod", err)
		}
		t2 := GetNowUnix()

		DebugInfo("updateTargetDir", (t2 - t1), " (sec)/", len(dirList))
	}
	return nil
}

func isCopyNeeded(fpath string, finfo fs.FileInfo, targetPath string) bool {
	if IsOverwrite == false && targetPath != "" {
		if FileExists(targetPath) {
			numStatistics["skip_exists"]++
			return false
		}
	}

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

	if ExcludeDir != "" {
		excludePath := strings.Replace(fpath, SourceDir, ExcludeDir, 1)
		if FileExists(excludePath) {
			numStatistics["skip_exclude_dir"]++
			DebugInfo("isCopyNeeded: SKIP", excludePath)
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

func taskChanFile() error {

	wgGetChanFile := sync.WaitGroup{}
	numWait := int32(qcap)
	curNumGet := int32(0)

	for {
		cf := <-chanFile
		if cf.CopyMode == -1 {
			isChanFileRWDone = true
			DebugInfo("_COPYSTATUS:chanFile:", "DONE")
			break
		}

		atomic.AddInt64(&totalWriteSize, cf.Finfo.Size())

		atomic.AddInt32(&taskNumGet, 1)
		wgGetChanFile.Add(1)

		go func(cf CopyElement) {
			defer func() {
				atomic.AddInt32(&taskNumGet, -1)
				wgGetChanFile.Done()
			}()
			getChanFileToDisk(cf)
		}(cf)

		curNumGet = atomic.LoadInt32(&taskNumGet)

		if curNumGet%numWait == 0 {
			wgGetChanFile.Wait()
		}

	}
	wgGetChanFile.Wait()

	return nil
}

func taskWalkSerial() error {
	var targetPath string
	filepath.Walk(SourceDir, func(fpath string, finfo os.FileInfo, err error) error {
		if err != nil {
			PrintError("taskWalkSerial", err)
			return err
		}
		fpath = ToUnixSlash(fpath)
		targetPath = ToUnixSlash(strings.Replace(fpath, SourceDir, TargetDir, 1))

		if finfo.IsDir() {
			if IsIgnoreEmptyFolder {
				return nil
			}
			D2Dir := strings.Replace(fpath, SourceDir, TargetDir, 1)
			MakeDirs(D2Dir)
			dirList[targetPath] = fs.FileInfoToDirEntry(finfo)
			return nil
		}

		totalNum++
		if finfo.Mode()&os.ModeSymlink != 0 {
			nl, err := copyLink(fpath, targetPath)
			if err == nil {
				if nl == 0 {
					numStatistics["skip_exists"]++
				} else {
					numStatistics["symbol_link"] += 1
				}
			}
			return nil
		}

		if isCopyNeeded(fpath, finfo, targetPath) == false {
			return nil
		}
		//
		_, err = copyFile(fpath, targetPath, finfo)
		if err != nil {
			PrintError("taskWalkSerial: copyFile", err)
			return err
		}
		atomic.AddInt64(&totalWriteSize, finfo.Size())
		if totalNum%50 == 0 {
			updateTotalSpeed()
		}

		return nil
	})
	return nil
}

func taskWalkAsync() error {
	var targetPath string
	filepath.WalkDir(SourceDir, func(fpath string, dirInfo fs.DirEntry, err error) error {
		if err != nil {
			PrintError("taskWalkAsync", err)
			return err
		}
		fpath = ToUnixSlash(fpath)
		targetPath = ToUnixSlash(strings.Replace(fpath, SourceDir, TargetDir, 1))

		if dirInfo.IsDir() {
			if IsIgnoreEmptyFolder {
				return nil
			}
			D2Dir := strings.Replace(fpath, SourceDir, TargetDir, 1)
			MakeDirs(D2Dir)
			dirList[targetPath] = dirInfo
			return nil
		}

		totalNum++
		if dirInfo.Type()&os.ModeSymlink != 0 {
			nl, err := copyLink(fpath, targetPath)
			if err == nil {
				if nl == 0 {
					numStatistics["skip_exists"]++
				} else {
					numStatistics["symbol_link"] += 1
				}
			}
			return nil
		}

		if MinSize != -1 && MaxSize != -1 && MinAge != "" && MaxAge != "" {
			if isCopyNeeded(fpath, nil, targetPath) == false {
				return nil
			}
		}

		finfo, err := dirInfo.Info()
		PrintError("taskWalkAsync", err)

		if isCopyNeeded(fpath, finfo, targetPath) == false {
			return nil
		}

		//
		ele, err := file2CopyElement(fpath, targetPath, finfo, 0)
		if err != nil {
			PrintError("taskWalkAsync: file2CopyElement", err)
			return err
		}
		chanFile <- ele

		return nil
	})
	return nil
}
