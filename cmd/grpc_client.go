package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	pb "pb"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func pbHeadSourceFiles() error {
	t1 := GetNowTime()
	var headCount int = 0
	var createCount int = 0
	var updateCount int = 0
	var fileHash map[string]string = make(map[string]string, 256)
	fextMatch = regexp.MustCompile("(?i)" + FileExt)
	SourceDir = ToUnixSlash(SourceDir)
	filepath.WalkDir(SourceDir, func(fpath string, dirInfo fs.DirEntry, err error) error {
		if err != nil {
			PrintError("pbHeadSourceFiles", err)
			return err
		}
		fpath = ToUnixSlash(fpath)

		if IsFollowSymlink == false {
			if isSymlink(fpath) {
				linkTo := getSymlink(fpath)
				if linkTo != "" {
					symList[fpath] = strings.Join([]string{"RAW", linkTo}, "::")
					if strings.HasPrefix(linkTo, SourceDir) {
						t1 := strings.TrimPrefix(linkTo, SourceDir)
						symList[fpath] = strings.Join([]string{"SUB", t1}, "::")
					}
				}
				return nil
			}
		}

		if dirInfo.IsDir() {
			if IsIgnoreEmptyFolder == false {
				dirList[fpath], err = dirInfo.Info()
				PrintError("pbHeadSourceFiles: dirInfo.Info", err)
			}
			return nil
		}

		if isCopyNeeded(fpath, dirInfo) == false {
			return nil
		}

		relPath := strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/")
		fileHash[relPath] = ""
		headCount++

		return nil
	})

	pbFile := NewPbFile()
	pbFile.Ftype = []byte("FileHashList")
	pbFile.Data = MapStr2Byte(fileHash)
	pbFile.Fsum = nil
	resp, err := gClient.Head(context.Background(), &pbFile)

	if err != nil {
		PrintError("pbHeadSourceFiles: gClient.Head", err)
		return err
	}

	var dhl map[string]string
	var diffHashList map[string]string
	if resp.Data != nil {
		diffHashList, err = Byte2MapStr(resp.Data, dhl)
		if err != nil {
			PrintError("pbHeadSourceFiles: Byte2MapStr", err)
			return err
		}
	}

	for spath, shash := range diffHashList {
		//DebugInfo("pbHeadSourceFiles", spath, " : ", shash)
		fpath := filepath.Join(SourceDir, spath)
		finfo, err := os.Stat(fpath)
		if err != nil {
			PrintError("pbHeadSourceFiles: os.Stat", err)
			continue
		}
		fsize := finfo.Size()
		sendFileList[spath] = fsize
		if shash == "404" {
			if fsize < smallFileSize {
				smallFileList = append(smallFileList, spath)
			} else if fsize >= smallFileSize && fsize < mediumFileSize64 {
				mediumFileList = append(mediumFileList, spath)
			} else {
				largeFileList = append(largeFileList, spath)
			}
			createCount++
			continue
		}

		if shash != hashFile(fpath) {
			fsize := finfo.Size()
			if fsize < smallFileSize {
				smallFileList = append(smallFileList, spath)
			} else if fsize >= smallFileSize && fsize < mediumFileSize64 {
				mediumFileList = append(mediumFileList, spath)
			} else {
				largeFileList = append(largeFileList, spath)
			}
			updateCount++
			continue
		}

	}

	DebugInfo("pbHeadSourceFiles: Duration", time.Since(t1), ", headCount: ", headCount)
	DebugInfo("pbHeadSourceFiles: createCount", createCount, ", updateCount: ", updateCount)
	DebugInfo("pbHeadSourceFiles: TotalCount", len(sendFileList))
	PrintlnInfo("green", "--------------------------------------", "")
	PrintlnInfo("green", "pbHeadSourceFiles: smallFileList", len(smallFileList))
	PrintlnInfo("green", "pbHeadSourceFiles: mediumFileList", len(mediumFileList))
	PrintlnInfo("green", "pbHeadSourceFiles: largeFileList", len(largeFileList))
	PrintlnInfo("green", "pbHeadSourceFiles: symlinkList", len(symList))
	PrintlnInfo("green", "--------------------------------------", "")

	sort.Strings(smallFileList)
	sort.Strings(mediumFileList)
	sort.Strings(largeFileList)
	dumpFileList(smallFileList, "rpcopy_send_smallFileList")
	dumpFileList(mediumFileList, "rpcopy_send_mediumFileList")
	dumpFileList(largeFileList, "rpcopy_send_largeFileList")
	//
	var dirs []string
	var syms []string
	for d, _ := range dirList {
		dirs = append(dirs, d)
	}
	for d, t := range symList {
		syms = append(syms, strings.Join([]string{d, t}, " -> "))
	}
	sort.Strings(dirs)
	sort.Strings(syms)
	dumpFileList(dirs, "rpcopy_send_dirList")
	dumpFileList(syms, "rpcopy_send_symlinkList")

	return nil
}

func ClientSendSmallFileList() error {
	var boltFileList []string
	var bsize int64 = 0
	var countBoltFileList int

	for _, spath := range smallFileList {
		bsize += sendFileList[spath]
		if bsize < maxBoltSize {
			boltFileList = append(boltFileList, spath)
		}

		if bsize >= maxBoltSize {
			DebugInfo("ClientSendFiles: bsize", bsize>>20, " MB, MaxBoltSize= ", maxBoltSize>>20, " MB")
			countBoltFileList = len(boltFileList)
			atomic.AddInt32(&totalNum, int32(countBoltFileList))
			atomic.AddInt64(&totalWriteSize, bsize)
			err := createBolt(boltFileList, strings.Join([]string{"rpcopy_client.db", Int2Str(countBoltFileList), Int64Str(GetNowUnixMilli())}, "_"))
			PrintError("ClientSendFiles:createBolt", err)
			boltFileList = boltFileList[:0]
			bsize = 0
		}
	}

	if len(boltFileList) > 0 {
		countBoltFileList = len(boltFileList)
		err := createBolt(boltFileList, strings.Join([]string{"rpcopy_client.db", Int2Str(countBoltFileList), Int64Str(GetNowUnixMilli())}, "_"))
		atomic.AddInt32(&totalNum, int32(countBoltFileList))
		atomic.AddInt64(&totalWriteSize, bsize)
		PrintError("ClientSendFiles:createBolt", err)
	}
	//

	return nil
}

func ClientSendMediumFileList() error {
	wg := sync.WaitGroup{}
	var count int = 0
	for _, spath := range mediumFileList {
		fpath := ToUnixSlash(filepath.Join(SourceDir, spath))
		finfo, err := os.Stat(fpath)
		if err != nil {
			PrintError("ClientSendMediumFileList", err)
			continue
		}
		pbFile := file2pbFile(fpath, finfo, "file")
		count++

		wg.Add(1)
		go func(fpath string, pbFile *pb.File) error {
			defer wg.Done()

			atomic.AddInt32(&totalNum, 1)
			atomic.AddInt64(&totalWriteSize, finfo.Size())
			//DebugInfo("ClientSendMediumFileList: Sending", fpath)
			err = pbFileSend(fpath, pbFile)
			PrintError("ClientSendMediumFileList: pbFileChunkSend", err)
			return nil
		}(fpath, pbFile)

		if count%4 == 0 {
			wg.Wait()
		}
	}
	wg.Wait()
	return nil
}

func ClientSendLargeFileList() error {
	wg := sync.WaitGroup{}
	var count int = 0
	for _, spath := range largeFileList {
		fpath := ToUnixSlash(filepath.Join(SourceDir, spath))
		finfo, err := os.Stat(fpath)
		if err != nil {
			PrintError("ClientSendLargeFileList", err)
			continue
		}
		pbFile := file2pbFile(fpath, finfo, "file")

		atomic.AddInt32(&totalNum, 1)
		atomic.AddInt64(&totalWriteSize, finfo.Size())
		DebugInfo("ClientSendLargeFileList: Sending", fpath)

		wg.Add(1)
		go func(fpath string, pbFile *pb.File) {
			defer wg.Done()
			PrintlnInfo("white", "Sending", fpath)
			err = pbFileChunkSend(fpath, pbFile)
			PrintError("ClientSendLargeFileList: pbFileChunkSend", err)
		}(fpath, pbFile)

		count++

		if count%2 == 0 {
			wg.Wait()
		}

	}
	wg.Wait()

	return nil
}

func ClientSendDirSymlink() error {
	DebugInfo("ClientSendFiles: Sending", "dir list")
	for k, v := range dirList {
		pbFile := file2pbFile(k, v, "dir")
		//
		err := gClientStream.Send(pbFile)
		if err != nil {
			PrintError("ClientSendFiles", err)
			continue
		}
	}

	//
	DebugInfo("ClientSendFiles: Sending", "sym list")
	for slink, sfile := range symList {
		pbFile := file2pbFile(slink, nil, "symlink")
		pbFile.Comment = []byte(sfile)
		//
		err := gClientStream.Send(pbFile)
		if err != nil {
			PrintError("ClientSendFiles", err)
			continue
		}
	}
	return nil
}

func ClientGetReport() error {
	DebugInfo("ClientGetReport", "Getting report ...")
	respMisc, err := gClient.GetMisc(context.Background(), &pb.Misc{Mtype: "pbSaveStatus"})
	if err != nil {
		PrintError("ClientGetReport:stream.Recv", err)
		return err
	}

	successTxt := filepath.Join(LogDir, GetNowTimeStr("Ymd"), strings.Join([]string{GetNowTimeStr("H"), "rpcopy", "success.log"}, "_"))
	failureTxt := filepath.Join(LogDir, GetNowTimeStr("Ymd"), strings.Join([]string{GetNowTimeStr("H"), "rpcopy", "error.log"}, "_"))
	MakeDirs(filepath.Dir(failureTxt))

	successWriter, err := os.OpenFile(successTxt, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	PrintError("ClientGetReport:os.OpenFile", err)
	failureWriter, err := os.OpenFile(failureTxt, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	PrintError("ClientGetReport:os.OpenFile", err)

	if err != nil {
		return err
	}

	var m map[string]string
	if respMisc.Data != nil {
		m2, err := Byte2MapStr(respMisc.Data, m)
		PrintError("ClientGetReport:Byte2MapStr", err)
		line := ""
		for k, v := range m2 {
			line = fmt.Sprintf("%s, %s \n", v, k)
			vint, err := strconv.Atoi(v)
			PrintError("ClientGetReport: strconv.Atoi", err)
			if vint > 199 && vint < 400 {
				successWriter.WriteString(line)
			} else {
				failureWriter.WriteString(line)
			}
		}
	}

	successWriter.Close()
	failureWriter.Close()

	DebugInfo("ClientGetReport", "Done.")

	return nil
}
