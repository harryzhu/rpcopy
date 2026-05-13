package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	pb "pb"
	"strings"
	"sync/atomic"
	"time"

	bolt "go.etcd.io/bbolt"
)

func NewPbFile() pb.File {
	return pb.File{
		Status:   0,
		Comment:  nil,
		ChanNum:  0,
		Path:     nil,
		Ftype:    nil,
		Finfo:    nil,
		Fsum:     nil,
		Fsize:    0,
		Chunks:   0,
		ChunkNum: 0,
		Zstd:     false,
		Data:     nil,
	}
}

func pbGetTargetPath(pbIn *pb.File) (dstPath string) {
	TargetDir = strings.TrimSuffix(ToUnixSlash(TargetDir), "/")
	pbInPath := strings.TrimPrefix(ToUnixSlash(string(pbIn.GetPath())), "/")
	dstPath = strings.Join([]string{TargetDir, pbInPath}, "/")

	return ToUnixSlash(dstPath)
}

func pbBoltSend(fpath string, pbFile *pb.File) error {
	fp, err := os.Open(fpath)
	if err != nil {
		PrintError("pbBoltSend:os.Open", err)
		return err
	}
	defer fp.Close()

	reader := bufio.NewReaderSize(fp, chunkSize)
	buffer := make([]byte, chunkSize)

	chunkTotal := pbFile.Chunks
	chunkNum := 0

	for {
		n, err := reader.Read(buffer)

		if err != nil && err != io.EOF {
			PrintError("pbBoltSend:reader.Read", err)
			return err
		}

		if n == 0 || err == io.EOF {
			break
		}

		pbFile.Data = buffer[:n]
		pbFile.Zstd = false
		pbFile.ChunkNum = int32(chunkNum)

		err = gClientStream.Send(pbFile)
		if err != nil {
			PrintError("pbBoltSend: gClientStream.Send", err)
			return err
		}

		DebugInfo("pbBoltSend", chunkNum, "/", chunkTotal, " : ", n)
		chunkNum++

	}

	DebugInfo("pbBoltSend", "DONE. ", pbFile.ChunkNum, "/", chunkTotal)

	return nil
}

func pbBoltSave(pbIn *pb.File) (boltPath string, err error) {
	boltPath = filepath.Join(LogDir, hashString([]byte(strings.Join([]string{"rpcopy_server.db", "bolt"}, "_"))))

	chunkNum := pbIn.GetChunkNum()
	totalChunks := pbIn.GetChunks()

	var boltWriter *os.File
	if chunkNum == 0 {
		boltWriter, err = os.OpenFile(boltPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	} else {
		boltWriter, err = os.OpenFile(boltPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	}

	if err != nil {
		PrintError("pbBoltSave: ", err)
		return "", err
	}
	defer boltWriter.Close()

	DebugInfo("pbBoltSave", chunkNum, "/", totalChunks, " : ", len(pbIn.Data))

	_, err = boltWriter.Write(pbIn.Data)
	if err != nil {
		PrintError("pbBoltSave: ", err)
		return "", err
	}
	if chunkNum < totalChunks-1 {
		return "", nil
	}

	if chunkNum == totalChunks-1 {
		PrintlnInfo("blue", "pbBoltSave", chunkNum, "/", totalChunks, " : ", len(pbIn.Data))
		boltWriter.Close()
		return boltPath, nil
	}

	return "", NewError("bolt cannot save successfully")
}

func pbBoltExtract(boltPath string) (statusCode int, err error) {
	db, err := bolt.Open(boltPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		PrintError("pbBoltExtract:Open", err)
		return 500, err
	}
	defer db.Close()

	t1 := time.Now()
	var key, fpath, dstPath, dkey string
	var infov, bv, fdata []byte
	var finfo FileInfoLite

	db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte("pbfiles"))
		c := bkt.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			key = string(k)
			if strings.HasPrefix(key, "INFO/") {
				fpath = strings.TrimPrefix(key, "INFO/")
				dstPath = ToUnixSlash(filepath.Join(TargetDir, fpath))
				if IsOverwrite == false {
					if FileExists(dstPath) {
						DebugInfo("pbBoltExtract", "SKIP as exist: ", fpath)
						continue
					}
				}
				//
				safePbSaveStatus.Store(fpath, int64(500))
				infov, err = UnZstdBytes(v)
				if err != nil {
					PrintError("pbBoltExtract:finfo:UnZstdBytes", err)
				}

				if infov != nil {
					finfo, err = fileInfoLite2Finfo(infov)
					if err != nil {
						PrintError("pbBoltExtract:fileInfoLite2Finfo", err)
					}
				}

				dkey = strings.Join([]string{"DATA", fpath}, "/")
				bv = bkt.Get([]byte(dkey))
				if bv != nil {
					fdata, err = UnZstdBytes(bv)
					if err != nil {
						PrintError("pbBoltExtract:fdata:UnZstdBytes", err)
					}
				}

				MakeDirs(filepath.Dir(dstPath))
				err := os.WriteFile(dstPath, fdata, os.ModePerm)
				if err != nil {
					PrintError("pbBoltExtract:os.WriteFile", err)
					return err
				}

				err1 := os.Chmod(dstPath, finfo.Mode)
				PrintError("pbBoltExtract:os.Chmod", err1)
				err2 := os.Chtimes(dstPath, finfo.ModTime, finfo.ModTime)
				PrintError("pbBoltExtract:os.Chtimes", err2)
				//
				if err1 != nil || err2 != nil {
					safePbSaveStatus.Store(fpath, int64(500))
				}

				if FileExists(dstPath) {
					safePbSaveStatus.Store(fpath, int64(201))
				}
			}
		}
		return nil
	})

	PrintlnInfo("green", "pbBoltExtract: Elapse:", time.Since(t1))
	db.Close()

	//
	if FileExists(boltPath) {
		err := os.Remove(boltPath)
		PrintError("pbBoltExtract:os.Remove", err)
		return 0, err
	}

	return 0, nil
}

func pbMiscBoltSaveOLD(pbIn *pb.Misc) (statusCode int, err error) {
	boltPath := filepath.Join(LogDir,
		hashString([]byte(strings.Join([]string{"rpcopy_server.db", Int64Str(GetNowUnixMilli())}, "_"))))
	if FileExists(boltPath) {
		err := os.Remove(boltPath)
		PrintError("pbMiscBoltSave:os.Remove", err)
		return 0, err
	}

	boltWriter, err := os.OpenFile(boltPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		PrintError("pbFileBoltSave: ", err)
		return 500, err
	}
	boltWriter.Truncate(0)
	DebugInfo("pbFileBoltSave: pbIn.Data", len(pbIn.Data))
	_, err = boltWriter.Write(pbIn.Data)
	if err != nil {
		PrintError("pbFileBoltSave: ", err)
		return 500, err
	}
	boltWriter.Close()

	db, err := bolt.Open(boltPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		PrintError("pbFileBoltSave:Open", err)
		return 500, err
	}
	defer db.Close()

	t1 := time.Now()
	var key, fpath, dstPath, dkey string
	var infov, bv, fdata []byte
	var finfo FileInfoLite

	db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte("pbfiles"))
		c := bkt.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			key = string(k)
			if strings.HasPrefix(key, "INFO/") {
				fpath = strings.TrimPrefix(key, "INFO/")
				dstPath = ToUnixSlash(filepath.Join(TargetDir, fpath))
				if IsOverwrite == false {
					if FileExists(dstPath) {
						DebugInfo("pbMiscBoltSave", "SKIP as exist: ", fpath)
						continue
					}
				}
				//
				safePbSaveStatus.Store(fpath, int64(500))
				infov, err = UnZstdBytes(v)
				if err != nil {
					PrintError("pbFileBoltSave:finfo:UnZstdBytes", err)
				}

				if infov != nil {
					finfo, err = fileInfoLite2Finfo(infov)
					if err != nil {
						PrintError("pbFileBoltSave:fileInfoLite2Finfo", err)
					}
				}

				dkey = strings.Join([]string{"DATA", fpath}, "/")
				bv = bkt.Get([]byte(dkey))
				if bv != nil {
					fdata, err = UnZstdBytes(bv)
					if err != nil {
						PrintError("pbFileBoltSave:fdata:UnZstdBytes", err)
					}
				}

				MakeDirs(filepath.Dir(dstPath))
				err := os.WriteFile(dstPath, fdata, os.ModePerm)
				if err != nil {
					PrintError("pbFileBoltSave:os.WriteFile", err)
					return err
				}

				err1 := os.Chmod(dstPath, finfo.Mode)
				PrintError("pbFileBoltSave:os.Chmod", err1)
				err2 := os.Chtimes(dstPath, finfo.ModTime, finfo.ModTime)
				PrintError("pbFileBoltSave:os.Chtimes", err2)
				//
				if err1 != nil || err2 != nil {
					safePbSaveStatus.Store(fpath, int64(500))
				}

				if FileExists(dstPath) {
					safePbSaveStatus.Store(fpath, int64(201))
				}
			}
		}
		return nil
	})

	PrintlnInfo("green", "pbMiscBoltSave: Elapse:", time.Since(t1))
	db.Close()

	//
	if FileExists(boltPath) {
		err := os.Remove(boltPath)
		PrintError("pbMiscBoltSave:os.Remove", err)
		return 0, err
	}

	return 0, nil
}

func finfo2FileInfoLite(finfo fs.FileInfo) []byte {
	filite := FileInfoLite{
		Size:    finfo.Size(),
		ModTime: finfo.ModTime(),
		Mode:    finfo.Mode(),
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(filite)
	PrintError("file2pbFile: enc.Encode", err)

	return buf.Bytes()
}

func fileInfoLite2Finfo(fi []byte) (filite FileInfoLite, err error) {
	if fi != nil {
		buf := bytes.NewBuffer(fi)
		dec := gob.NewDecoder(buf)
		err := dec.Decode(&filite)
		if err != nil {
			PrintError("fileInfoLite2Finfo: gob.NewDecoder", err)
			return filite, err
		}
		return filite, nil
	}
	return filite, NewError("fi cannot be empty")
}

func file2pbFile(fpath string, finfo fs.FileInfo, ftype string) *pb.File {
	pbFile := &pb.File{}
	//
	pbFile.Status = 0
	pbFile.Comment = nil
	if fpath == "" {
		pbFile.Path = nil
	} else {
		pbFile.Path = []byte(strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/"))
	}
	pbFile.Ftype = []byte(ftype)
	pbFile.ChunkNum = 0
	pbFile.Data = nil
	if ftype == "file" {
		pbFile.Fsum = []byte(hashFile(fpath))
	} else {
		pbFile.Fsum = nil
	}
	//
	if finfo == nil {
		pbFile.Chunks = 0
		pbFile.Fsize = 0
		pbFile.Finfo = nil
	} else {
		pbFile.Chunks = int32(math.Ceil(float64(finfo.Size()) / float64(chunkSize)))
		pbFile.Fsize = finfo.Size()
		pbFile.Finfo = finfo2FileInfoLite(finfo)
	}

	return pbFile
}

func serverHealthCheck() error {
	sp1 := GetNowTime()
	respMisc, err := gClient.GetMisc(context.Background(), &pb.Misc{Mtype: "ping", Data: []byte(strings.Join([]string{"ping from", Host}, " <= "))})

	if err != nil {
		PrintError("serverHealthCheck:", err)
		return err
	}
	var rt string
	if string(respMisc.Data) == "pong" {
		rt = "pong"
	}

	if string(respMisc.Data) == "error" {
		rt = "error"
	}
	PrintlnInfo("Cyan", "HealthCheck From Server", rt, ". [Latency]: ", time.Since(sp1))
	return nil
}

func pbFileSend(fpath string, pbFile *pb.File) error {
	pbFileData, err := os.ReadFile(fpath)
	if err != nil {
		PrintError("pbFileSend:os.ReadFile", err)
		return err
	}

	atomic.AddInt32(&chanNum, 1)
	atomic.AddInt64(&totalWriteSize, int64(pbFile.Size()))

	if IsZstdSend {
		pbFile.Data = ZstdBytes(pbFileData)
		pbFile.Zstd = true
	} else {
		pbFile.Data = pbFileData
		pbFile.Zstd = false
	}

	_, err = gClient.Put(context.Background(), pbFile)
	if err != nil {
		PrintError("pbFileSend: gClientStream.Send", err)
		return err
	}

	DebugInfo("pbFileSend", "ONE_DONE")

	return nil
}

func pbFileSave(pbIn *pb.File) (statusCode int, err error) {
	dstPath := ToUnixSlash(pbGetTargetPath(pbIn))
	dstPathTemp := ToUnixSlash(strings.Join([]string{dstPath, "ing"}, "."))

	fi := pbIn.GetFinfo()

	var pbInFinfo FileInfoLite
	pbInFinfo, err = fileInfoLite2Finfo(fi)
	if err != nil {
		PrintError("pbFileChunkSave: fileInfoLite2Finfo", err)
		return 500, err
	}

	MakeDirs(filepath.Dir(dstPath))

	dstWriter, err := os.OpenFile(dstPathTemp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	dstWriter.Truncate(0)

	var pbInData []byte
	if pbIn.Zstd == true {
		pbInData, err = UnZstdBytes(pbIn.Data)
		if err != nil {
			PrintError("pbFileChunkSave: UnZstdBytes", err)
			return 500, err
		}
	} else {
		pbInData = pbIn.Data
	}

	_, err = dstWriter.Write(pbInData)
	if err != nil {
		PrintError("pbFileChunkSave: dstWriter.Write", err)
		return 500, err
	}
	dstWriter.Close()

	pbInData = nil

	dstSum := hashFile(dstPathTemp)
	if string(pbIn.GetFsum()) != dstSum {
		err = NewError("dstPath xxhash is not matched")
		PrintError("streamSaveFile: dstSum", err)
		return 500, err
	}

	DebugInfo("streamSaveFile: dstSum matched", dstSum)
	if err = os.Rename(dstPathTemp, dstPath); err != nil {
		PrintError("streamSaveFile: os.Rename", err)
		return 500, err
	}

	if err = os.Chmod(dstPath, pbInFinfo.Mode); err != nil {
		PrintError("streamSaveFile: os.Chmod", err)
		return 500, err
	}

	if err = os.Chtimes(dstPath, pbInFinfo.ModTime, pbInFinfo.ModTime); err != nil {
		PrintError("streamSaveFile: os.Chtimes", err)
		return 500, err
	}

	return 206, nil
}

func pbFileChunkSend(fpath string, pbFile *pb.File) error {
	fp, err := os.Open(fpath)
	if err != nil {
		PrintError("pbFileChunkSend:os.Open", err)
		return err
	}
	defer fp.Close()

	reader := bufio.NewReaderSize(fp, chunkSize)
	buffer := make([]byte, chunkSize)

	atomic.AddInt32(&chanNum, 1)

	channum := atomic.LoadInt32(&chanNum)
	if channum > 3 {
		atomic.StoreInt32(&chanNum, 0)
		channum = 0
	}

	chunkTotal := pbFile.Chunks
	chunkNum := 0

	for {
		n, err := reader.Read(buffer)

		if err != nil && err != io.EOF {
			PrintError("pbFileChunkSend:reader.Read", err)
			return err
		}

		if n == 0 || err == io.EOF {
			break
		}

		if IsZstdSend {
			pbFile.Data = ZstdBytes(buffer[:n])
			pbFile.Zstd = true
		} else {
			pbFile.Data = buffer[:n]
			pbFile.Zstd = false
		}

		pbFile.ChunkNum = int32(chunkNum)
		pbFile.ChanNum = channum

		err = gClientStream.Send(pbFile)
		if err != nil {
			PrintError("pbFileChunkSend: gClientStream.Send", err)
			return err
		}

		DebugInfo("pbFileChunkSend: chanNum", pbFile.ChanNum, " <- chunk: ", pbFile.ChunkNum, "/", chunkTotal, " : ", n)
		chunkNum++

	}

	DebugInfo("pbFileChunkSend", "ONE_DONE. ", pbFile.ChunkNum, "/", chunkTotal)

	return nil
}

func pbFileChunkSave(pbIn *pb.File) (statusCode int, err error) {
	DebugInfo("--- pbFileChunkSave: Received", pbIn.GetChanNum(), " <- ", pbIn.GetChunkNum(), "/", pbIn.GetChunks(), ": ", len(pbIn.Data))
	if TargetDir == "" {
		return 500, NewError("TargetDir cannot be empty")
	}

	if pbIn.Path == nil {
		return 400, NewError("pbIn.Path cannot be empty")
	}

	if pbIn.Status < 0 {
		return int(pbIn.Status), nil
	}

	dstPath := ToUnixSlash(pbGetTargetPath(pbIn))
	dstPathTemp := ToUnixSlash(strings.Join([]string{dstPath, "ing"}, "."))

	fi := pbIn.GetFinfo()

	var pbInFinfo FileInfoLite
	pbInFinfo, err = fileInfoLite2Finfo(fi)
	if err != nil {
		PrintError("pbFileChunkSave: fileInfoLite2Finfo", err)
		return 500, err
	}

	MakeDirs(filepath.Dir(dstPath))

	dstWriter, err := os.OpenFile(dstPathTemp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	if pbIn.GetChunkNum() == 0 {
		dstWriter.Truncate(0)
	}
	defer dstWriter.Close()

	if err != nil {
		PrintError("pbFileChunkSave: os.OpenFile", err)
		return 500, err
	}

	chunkTotal := pbIn.GetChunks()
	chunkNum := pbIn.GetChunkNum()

	DebugInfo("--- pbFileChunkSave", chunkNum, "/", chunkTotal)
	if pbIn.Data == nil {
		PrintError("pbFileChunkSave", NewError("pbIn.Data cannot be empty"))
		return 500, NewError("pbIn.Data cannot be empty")
	}

	var pbInData []byte
	if pbIn.Zstd == true {
		pbInData, err = UnZstdBytes(pbIn.Data)
		if err != nil {
			PrintError("pbFileChunkSave: UnZstdBytes", err)
			return 500, err
		}
	} else {
		pbInData = pbIn.Data
	}

	_, err = dstWriter.Write(pbInData)
	if err != nil {
		PrintError("pbFileChunkSave: dstWriter.Write", err)
		return 500, err
	}

	if chunkNum == chunkTotal-1 {
		dstWriter.Close()

		dstSum := hashFile(dstPathTemp)
		if string(pbIn.GetFsum()) != dstSum {
			err = NewError("dstPath xxhash is not matched")
			PrintError("streamSaveFile: dstSum", err)
			return 500, err
		}

		DebugInfo("streamSaveFile: dstSum matched", dstSum)
		if err = os.Rename(dstPathTemp, dstPath); err != nil {
			PrintError("streamSaveFile: os.Rename", err)
			return 500, err
		}

		if err = os.Chmod(dstPath, pbInFinfo.Mode); err != nil {
			PrintError("streamSaveFile: os.Chmod", err)
			return 500, err
		}

		if err = os.Chtimes(dstPath, pbInFinfo.ModTime, pbInFinfo.ModTime); err != nil {
			PrintError("streamSaveFile: os.Chtimes", err)
			return 500, err
		}
		DebugInfo("streamSaveFile: Saved", chunkTotal, ": ", string(pbIn.Path))
		DebugInfo("streamSaveFile: chanFile0123", len(chanFile), ", ", len(chanFile1), ", ", len(chanFile2), ", ", len(chanFile3))

		return 200, nil
	}

	return 206, nil
}

func pbFileDirSymlinkSave(pbIn *pb.File) (respStatus int32) {
	if string(pbIn.Ftype) == "dir" {
		dstPath := filepath.Join(TargetDir, string(pbIn.GetPath()))
		//DebugInfo("dir", dstPath)
		MakeDirs(dstPath)

		if pbIn.Finfo != nil {
			finfo, err := fileInfoLite2Finfo(pbIn.Finfo)
			if err == nil {
				err = os.Chmod(dstPath, finfo.Mode)
				PrintError("StreamReceive:dir:os.Chmod", err)
				err = os.Chtimes(dstPath, finfo.ModTime, finfo.ModTime)
				PrintError("StreamReceive:dir:os.Chtimes", err)
			}
		}

		respStatus = 206
		return respStatus
	}

	if string(pbIn.Ftype) == "symlink" {
		pbInLink := string(pbIn.GetPath())
		pbInFile := string(pbIn.GetComment())
		pre := pbInFile[0:5]

		symLink := filepath.Join(TargetDir, pbInLink)

		var srcFile string
		if pre == "RAW::" {
			srcFile = strings.TrimPrefix(pbInFile, "RAW::")
		}
		if pre == "SUB::" {
			srcFile = filepath.Join(TargetDir, strings.TrimPrefix(pbInFile, "SUB::"))
		}

		DebugInfo("symlink", pre, " => ", symLink, " => ", srcFile)
		MakeDirs(filepath.Dir(symLink))
		MakeSymlink(srcFile, symLink)

		respStatus = 206
		return respStatus
	}

	return 206
}
