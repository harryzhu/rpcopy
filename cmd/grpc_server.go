package cmd

import (
	"context"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	pb "pb"
	"strings"
	"time"

	"google.golang.org/grpc"
)

var (
	chanFile  chan *pb.File = make(chan *pb.File, 4)
	chanFile1 chan *pb.File = make(chan *pb.File, 4)
	chanFile2 chan *pb.File = make(chan *pb.File, 4)
	chanFile3 chan *pb.File = make(chan *pb.File, 4)
	//
	pbSaveStatus map[string]int = make(map[string]int, 1024)
)

type FileInfoLite struct {
	Size    int64
	ModTime time.Time
	Mode    fs.FileMode
}

type FileTransferService struct{}

func (s *FileTransferService) Head(ctx context.Context, pbIn *pb.File) (*pb.File, error) {
	resp := NewPbFile()
	resp.Path = pbIn.GetPath()
	resp.Status = 0
	dstPath := pbGetTargetPath(pbIn)
	if FileExists(dstPath) {
		resp.Status = 200
	}

	if IsOverwrite {
		resp.Status = 0
	}
	return &resp, nil
}

func (s *FileTransferService) GetMisc(ctx context.Context, pbIn *pb.Misc) (*pb.Misc, error) {
	resp := &pb.Misc{}
	resp.Mtype = pbIn.Mtype
	resp.Data = nil

	reqType := pbIn.Mtype
	switch reqType {
	case "pbSaveStatus":
		resp.Data = Map2Byte(pbSaveStatus)
	default:
		resp.Data = []byte("ok")
	}

	return resp, nil
}

func (s *FileTransferService) StreamReceive(stream pb.FileTransfer_StreamReceiveServer) error {
	for {
		resp := NewPbFile()

		pbIn, err := stream.Recv()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			PrintError("StreamReceive:err!=nil", err)
			resp.Status = 500
			resp.Comment = []byte(err.Error())
			return stream.Send(&resp)
		}

		if pbIn == nil {
			resp.Status = 501
			resp.Comment = []byte("StreamReceive:pbIn==nil")
			return stream.Send(&resp)
		}

		resp.Path = pbIn.Path

		reqType := string(pbIn.Ftype)

		if reqType == "file" {
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

		if reqType == "dir" || reqType == "symlink" {
			resp.Status = pbFileDirSymlinkSave(pbIn)
		}

		if reqType == "SIG" {
			DebugInfo("StreamReceive=======", pbIn.Status)
			if pbIn.Status == -10 {
				resp_10 := NewPbFile()
				stat := Map2Byte(pbSaveStatus)
				resp_10.Status = 2
				resp_10.Data = stat
				DebugInfo("StreamReceive: Map2Byte", len(stat))

				return stream.Send(&resp_10)
			}

		}

		DebugInfo("StreamReceive: resp.Status", resp.Status)

		err = stream.Send(&resp)
		if err != nil {
			PrintError("StreamReceive", err)
		}

	}
	return nil
}

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

func pbFileDirSymlinkSave(pbIn *pb.File) (respStatus int32) {
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

func pbFileChunkSave(pbIn *pb.File) (statusCode int, err error) {
	if TargetDir == "" {
		return 500, NewError("TargetDir cannot be empty")
	}

	if pbIn.Path == nil {
		return 400, NewError("pbIn.Path cannot be empty")
	}

	if pbIn.Status < 0 {
		return int(pbIn.Status), nil
	}

	dstPath := pbGetTargetPath(pbIn)
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
		return 500, NewError("pbIn.Data cannot be empty")
	}

	var pbInData []byte
	if pbIn.Zstd == true {
		pbInData, err = UnZstdBytes(pbIn.Data)
	} else {
		pbInData = pbIn.Data
	}

	if err != nil {
		PrintError("pbFileChunkSave: pbIn.Data", err)
		return 500, err
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

		return 200, nil
	}

	return 206, nil
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
