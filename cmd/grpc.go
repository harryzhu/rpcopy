package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"io"
	"io/fs"
	"math"
	"net"
	"os"
	"path/filepath"
	pb "pb"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
)

var (
	chanFile         chan *pb.File = make(chan *pb.File, 32)
	chanFile1        chan *pb.File = make(chan *pb.File, 32)
	chanFile2        chan *pb.File = make(chan *pb.File, 32)
	chanFile3        chan *pb.File = make(chan *pb.File, 32)
	isChanFileRWDone bool
)

type FileInfoLite struct {
	Size    int64
	ModTime time.Time
	Mode    fs.FileMode
}

type FileTransferService struct{}

func (s *FileTransferService) Head(ctx context.Context, pbIn *pb.File) (*pb.File, error) {
	resp := &pb.File{Path: pbIn.GetPath(), Status: 0}
	dstPath := pbGetTargetPath(pbIn)
	if FileExists(dstPath) {
		resp.Status = 200
	}

	if IsOverwrite {
		resp.Status = 0
	}
	return resp, nil
}

func (s *FileTransferService) StreamReceive(stream pb.FileTransfer_StreamReceiveServer) error {
	for {
		pbIn, err := stream.Recv()
		if err == io.EOF {
			return nil
		}

		if pbIn == nil {
			continue
		}

		resp := pb.File{}

		if string(pbIn.Ftype) == "file" {
			switch pbIn.ChanNum {
			case 1:
				chanFile1 <- pbIn
			case 2:
				chanFile2 <- pbIn
			case 3:
				chanFile3 <- pbIn
			default:
				chanFile <- pbIn
			}

			resp.Status = 201
		}

		if string(pbIn.Ftype) == "dir" {
			dstPath := filepath.Join(TargetDir, string(pbIn.GetPath()))
			DebugInfo("dir", dstPath)
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

			resp.Status = 206
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

			resp.Status = 206
		}

		DebugInfo("StreamReceive", resp.Status)

		err = stream.Send(&resp)
		if err != nil {
			PrintError("StreamReceive", err)
		}

	}
	return nil
}

func pbGetTargetPath(pbIn *pb.File) (dstPath string) {
	TargetDir = strings.TrimSuffix(ToUnixSlash(TargetDir), "/")
	pbInPath := strings.TrimPrefix(ToUnixSlash(string(pbIn.GetPath())), "/")
	dstPath = strings.Join([]string{TargetDir, pbInPath}, "/")

	return ToUnixSlash(dstPath)
}

func pbFileChunkSave(pbIn *pb.File) (success bool, err error) {
	if TargetDir == "" || pbIn.Path == nil {
		return false, NewError("TargetDir or pbIn.Path cannot be empty")
	}

	if pbIn.Status < 0 {
		return true, nil
	}

	dstPath := pbGetTargetPath(pbIn)
	dstPathTemp := ToUnixSlash(strings.Join([]string{dstPath, "ing"}, "."))

	fi := pbIn.GetFinfo()

	var pbInFinfo FileInfoLite
	pbInFinfo, err = fileInfoLite2Finfo(fi)
	if err != nil {
		PrintError("pbFileChunkSave: fileInfoLite2Finfo", err)
		return false, err
	}

	MakeDirs(filepath.Dir(dstPath))

	dstWriter, err := os.OpenFile(dstPathTemp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	if pbIn.GetChunkNum() == 0 {
		dstWriter.Truncate(0)
	}
	defer dstWriter.Close()

	if err != nil {
		PrintError("pbFileChunkSave: os.OpenFile", err)
		return false, err
	}

	chunkTotal := pbIn.GetChunks()
	chunkNum := pbIn.GetChunkNum()

	DebugInfo("--- pbFileChunkSave", chunkNum, "/", chunkTotal)
	if pbIn.Data == nil {
		return false, NewError("pbIn.Data cannot be empty")
	}

	var pbInData []byte
	if pbIn.Zstd == true {
		pbInData, err = UnZstdBytes(pbIn.Data)
	} else {
		pbInData = pbIn.Data
	}

	if err != nil {
		PrintError("pbFileChunkSave: pbIn.Data", err)
		return false, err
	}

	_, err = dstWriter.Write(pbInData)
	if err != nil {
		PrintError("pbFileChunkSave: dstWriter.Write", err)
		return false, err
	}

	if chunkNum == chunkTotal-1 {
		dstWriter.Close()

		dstSum := hashFile(dstPathTemp)
		if string(pbIn.GetFsum()) != dstSum {
			err = NewError("dstPath xxhash is not matched")
			PrintError("streamSaveFile: dstSum", err)
			return false, err
		}

		DebugInfo("streamSaveFile: dstSum matched", dstSum)
		if err = os.Rename(dstPathTemp, dstPath); err != nil {
			PrintError("streamSaveFile: os.Rename", err)
			return false, err
		}

		if err = os.Chmod(dstPath, pbInFinfo.Mode); err != nil {
			PrintError("streamSaveFile: os.Chmod", err)
			return false, err
		}

		if err = os.Chtimes(dstPath, pbInFinfo.ModTime, pbInFinfo.ModTime); err != nil {
			PrintError("streamSaveFile: os.Chtimes", err)
			return false, err
		}
		return true, nil
	}

	return false, nil
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

func pbFileChunkSend(fpath string, pbFile *pb.File, stream pb.FileTransfer_StreamReceiveClient) error {
	fp, err := os.Open(fpath)
	if err != nil {
		PrintError("pbFileChunkSend", err)
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
			return nil
		}

		if n == 0 {
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

		err = stream.Send(pbFile)
		if err != nil {
			PrintError("pbFileChunkSend", err)
			return err
		}

		resp, err := stream.Recv()
		if err != nil {
			PrintError("pbFileChunkSend", err)
			return err
		}
		if resp.Status == 200 {
			return nil
		}

		DebugInfo("pbFileChunkSend: Chunk", chunkNum, " / ", chunkTotal, " : ", n)
		chunkNum++

		if err == io.EOF {
			break
		}
	}

	DebugInfo("pbFileChunkSend: ONE_DONE", chunkNum, " / ", chunkTotal)

	return nil
}

func StartFileTransferServer() {
	hostPort := strings.Join([]string{Host, Port}, ":")
	listening, err := net.Listen("tcp", hostPort)
	if err != nil {
		PrintError("StartFileTransferServer: ", err)
	} else {
		PrintlnInfo("Endpoint RPC: ", hostPort)
	}

	grpcServerFileTransfer := grpc.NewServer(
		grpc.MaxMsgSize(MaxMessageSize),
		grpc.MaxRecvMsgSize(MaxMessageSize),
		grpc.MaxSendMsgSize(MaxMessageSize))

	pb.RegisterFileTransferServer(grpcServerFileTransfer, &FileTransferService{})

	grpcServerFileTransfer.Serve(listening)
}

func pbFileHead(pbFile *pb.File, clientHead pb.FileTransferClient) *pb.File {
	resp, err := clientHead.Head(context.Background(), pbFile)
	PrintError("pbFileHead", err)
	return resp
}

func ClientSendFiles() error {
	hostPort := strings.Join([]string{Host, Port}, ":")
	conn, err := grpc.Dial(hostPort, grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize), grpc.MaxCallSendMsgSize(MaxMessageSize)))
	if err != nil {
		PrintError("ClientSendFiles", err)
	}
	defer conn.Close()

	var client pb.FileTransferClient
	client = pb.NewFileTransferClient(conn)
	if client == nil {
		return nil
	}
	//
	clientStream, err := client.StreamReceive(context.Background())
	if err != nil {
		PrintError("ClientSendFiles", err)
	}

	SourceDir = ToUnixSlash(SourceDir)

	wgSend := sync.WaitGroup{}
	filepath.Walk(SourceDir, func(fpath string, finfo fs.FileInfo, err error) error {
		if err != nil {
			PrintError("ClientSendFiles: filepath.Walk", err)
			return err
		}

		fpath = ToUnixSlash(fpath)

		if finfo.IsDir() {
			if IsIgnoreEmptyFolder == false {
				dirList[fpath] = finfo
			}
			return nil
		}

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

		if isCopyNeeded(fpath, finfo) == false {
			return nil
		}

		pbFile := file2pbFile(fpath, finfo, "file")
		if IsOverwrite == false {
			pbRemote := pbFileHead(pbFile, client)

			if pbRemote == nil {
				return nil
			}

			if pbRemote.Status == 200 {
				DebugInfo("ClientSendFiles: SKIP", string(pbRemote.Path))
				return nil
			}
		}

		totalNum++
		atomic.AddInt64(&totalWriteSize, finfo.Size())

		wgSend.Add(1)
		go func(fpath string, pbFile *pb.File, stream pb.FileTransfer_StreamReceiveClient) error {
			defer wgSend.Done()
			pbFileChunkSend(fpath, pbFile, clientStream)
			return nil
		}(fpath, pbFile, clientStream)

		if totalNum > 15 && totalNum%16 == 0 {
			wgSend.Wait()
		}

		tws := atomic.LoadInt64(&totalWriteSize)
		PrintSpinner(strings.Join([]string{Int2Str(totalNum), " => ", Int64Str(tws >> 20), "MB"}, ""))

		return nil
	})

	wgSend.Wait()

	//
	for k, v := range dirList {
		pbFile := file2pbFile(k, v, "dir")
		//
		err = clientStream.Send(pbFile)
		if err != nil {
			PrintError("ClientSendFiles", err)
			return err
		}

		_, err := clientStream.Recv()
		if err != nil {
			PrintError("ClientSendFiles", err)
			return err
		}
	}

	//
	for slink, sfile := range symList {
		pbFile := file2pbFile(slink, nil, "symlink")
		pbFile.Comment = []byte(sfile)
		//
		err = clientStream.Send(pbFile)
		if err != nil {
			PrintError("ClientSendFiles", err)
			return err
		}

		_, err := clientStream.Recv()
		if err != nil {
			PrintError("ClientSendFiles", err)
			return err
		}
	}
	//
	sendFinishSignal(clientStream)
	//
	time.Sleep(1 * time.Second)
	err = clientStream.CloseSend()
	if err != nil {
		PrintError("ClientSendFiles: CloseSend", err)
		return err
	}

	return nil
}

func sendFinishSignal(clientStream pb.FileTransfer_StreamReceiveClient) error {
	pbFinish := &pb.File{Status: -1, Ftype: []byte("file")}

	pbFinish.ChanNum = 0
	err := clientStream.Send(pbFinish)
	if err != nil {
		PrintError("sendFinishSignal", err)
		return err
	}

	pbFinish.ChanNum = 1
	err = clientStream.Send(pbFinish)
	if err != nil {
		PrintError("sendFinishSignal", err)
		return err
	}

	pbFinish.ChanNum = 2
	err = clientStream.Send(pbFinish)
	if err != nil {
		PrintError("sendFinishSignal", err)
		return err
	}

	pbFinish.ChanNum = 3
	err = clientStream.Send(pbFinish)
	if err != nil {
		PrintError("sendFinishSignal", err)
		return err
	}
	return nil
}
