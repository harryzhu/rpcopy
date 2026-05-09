package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	pb "pb"
	"strings"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	chanFile  chan *pb.File = make(chan *pb.File, 4)
	chanFile1 chan *pb.File = make(chan *pb.File, 4)
	chanFile2 chan *pb.File = make(chan *pb.File, 4)
	chanFile3 chan *pb.File = make(chan *pb.File, 4)
	//
	safePbSaveStatus sync.Map
	pbSaveStatus     map[string]int64 = make(map[string]int64, 2048)
	//
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
	resp.Data = nil
	dstPath := pbGetTargetPath(pbIn)
	if FileExists(dstPath) {
		resp.Status = 200
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
		safePbSaveStatus.Range(func(k, v any) bool {
			pbSaveStatus[k.(string)] = v.(int64)
			return true
		})
		resp.Data = Map2Byte(pbSaveStatus)
	case "targetFileList":
		targetDirFileList := GetFileList(TargetDir, false)
		if IsOverwrite {
			targetDirFileList = make(map[string]int64, 0)
		}
		resp.Data = Map2Byte(targetDirFileList)
	case "ping":
		PrintlnInfo("HealthCheck from Client", string(pbIn.Data))
		if strings.Contains(string(pbIn.Data), "ping") {
			resp.Data = []byte("pong")
		} else {
			resp.Data = []byte("error")
		}

	default:
		resp.Data = []byte("ok")
	}

	return resp, nil
}

func (s *FileTransferService) PutMisc(ctx context.Context, pbIn *pb.Misc) (*pb.Misc, error) {
	resp := &pb.Misc{}
	resp.Mtype = pbIn.Mtype
	resp.Data = nil

	reqType := pbIn.Mtype
	switch reqType {
	case "pbBolt":
		pbMiscBoltSave(pbIn)
	default:
		resp.Data = []byte("ok")
	}

	return resp, nil
}

func (s *FileTransferService) StreamReceive(stream pb.FileTransfer_StreamReceiveServer) error {
	emptyPbFile := NewPbFile()
	for {
		pbIn, err := stream.Recv()
		if err == io.EOF {
			//DebugInfo("StreamReceive", err)
			//continue
			return nil
		}

		if err != nil {
			//PrintError("StreamReceive:err!=nil", err)
			continue
		}

		if pbIn == nil {
			PrintError("StreamReceive", NewError("pbIn==nil"))
			continue
		}

		resp := emptyPbFile

		//DebugInfo("StreamReceive: pbInPath", string(pbIn.Path))

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

			continue
		}

		if reqType == "dir" || reqType == "symlink" {
			resp.Status = pbFileDirSymlinkSave(pbIn)
			continue
		}

		if reqType == "SIG" {
			DebugInfo("StreamReceive:SIG", pbIn.Status)
			if pbIn.Status == -10 {
				resp_10 := NewPbFile()
				stat := Map2Byte(pbSaveStatus)
				resp_10.Status = 2
				resp_10.Data = stat
				DebugInfo("StreamReceive: Map2Byte", len(stat))

				stream.Send(&resp_10)
				continue
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

func pbMiscBoltSave(pbIn *pb.Misc) (statusCode int, err error) {
	boltPath, _ := filepath.Abs(filepath.Join(LogDir,
		hashString([]byte(strings.Join([]string{"rpcopy_server.db", Int64Str(GetNowUnixMilli())}, "_")))))
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
	//boltWriter.Truncate(0)
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
	wg := sync.WaitGroup{}
	db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte("pbfiles"))
		c := bkt.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			key := string(k)
			if strings.HasPrefix(key, "INFO/") {
				fpath := strings.TrimPrefix(key, "INFO/")
				safePbSaveStatus.Store(fpath, int64(500))
				infov, err := UnZstdBytes(v)
				if err != nil {
					PrintError("pbFileBoltSave:finfo:UnZstdBytes", err)
				}

				var finfo FileInfoLite
				if infov != nil {
					finfo, err = fileInfoLite2Finfo(infov)
					if err != nil {
						PrintError("pbFileBoltSave:fileInfoLite2Finfo", err)
					}
				}

				dkey := strings.Join([]string{"DATA", fpath}, "/")
				var fdata []byte
				bv := bkt.Get([]byte(dkey))
				if bv != nil {
					fdata, err = UnZstdBytes(bv)
					if err != nil {
						PrintError("pbFileBoltSave:fdata:UnZstdBytes", err)
					}
				}

				dstPath := ToUnixSlash(filepath.Join(TargetDir, fpath))

				wg.Add(1)
				go func(dstPath string, fdata []byte, finfo FileInfoLite) error {
					defer wg.Done()

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
					return nil
				}(dstPath, fdata, finfo)

			}

		}
		return nil
	})
	wg.Wait()
	PrintlnInfo("pbMiscBoltSave: Elapse:", time.Since(t1))

	db.Close()
	if FileExists(boltPath) {
		err := os.Remove(boltPath)
		PrintError("pbMiscBoltSave:os.Remove", err)
		return 0, err
	}

	return 0, nil
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
		DebugInfo("streamSaveFile: chanFile0123", len(chanFile), ", ", len(chanFile1), ", ", len(chanFile2), ", ", len(chanFile3))

		return 200, nil
	}

	return 206, nil
}

func StartFileTransferServer() {
	hostPort := strings.Join([]string{Host, Port}, ":")
	listening, err := net.Listen("tcp", hostPort)
	if err != nil {
		FatalError("StartFileTransferServer: ", err)
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

func StartTLSFileTransferServer() {
	certificate, err := tls.LoadX509KeyPair("cert/server/server.crt", "cert/server/server.key")
	if err != nil {
		FatalError("StartTLSFileTransferServer:tls.LoadX509KeyPair", err)
	}

	certPool := x509.NewCertPool()
	ca, err := os.ReadFile("cert/ca.crt")
	if err != nil {
		FatalError("StartTLSFileTransferServer:os.ReadFile", err)
	}

	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		FatalError("StartTLSFileTransferServer:os.ReadFile", NewError("certPool.AppendCertsFromPEM"))
	}

	opts := []grpc.ServerOption{
		grpc.Creds(credentials.NewTLS(&tls.Config{
			ClientAuth:   tls.RequireAndVerifyClientCert,
			Certificates: []tls.Certificate{certificate},
			ClientCAs:    certPool,
		},
		)),
		grpc.MaxMsgSize(MaxMessageSize),
		grpc.MaxRecvMsgSize(MaxMessageSize),
		grpc.MaxSendMsgSize(MaxMessageSize),
	}

	hostPort := strings.Join([]string{Host, Port}, ":")
	listening, err := net.Listen("tcp", hostPort)
	if err != nil {
		FatalError("StartFileTransferServer: ", err)
	} else {
		PrintlnInfo("Endpoint RPC: ", hostPort)
	}

	grpcServerFileTransfer := grpc.NewServer(opts...)

	pb.RegisterFileTransferServer(grpcServerFileTransfer, &FileTransferService{})

	grpcServerFileTransfer.Serve(listening)
}
