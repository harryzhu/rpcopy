package cmd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/gob"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	pb "pb"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	chanChunkSendStatus chan int = make(chan int, 1)
	gClient             pb.FileTransferClient
	gClientStream       pb.FileTransfer_StreamReceiveClient
	gClientConn         *grpc.ClientConn
	//
	sendFileList  map[string]int64
	smallFileList []string
	largeFileList []string
	//
	progressFlag int32 = 0
)

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
	if ftype == "file" || ftype == "bolt" {
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
	//channum = 0
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

		err = stream.Send(pbFile)
		if err != nil {
			PrintError("pbFileChunkSend:stream.Send", err)
			return err
		}

		DebugInfo("pbFileChunkSend: ChanNum", chunkNum, " / ", chunkTotal, " : ", n)
		chunkNum++

	}

	DebugInfo("pbFileChunkSend", "ONE_DONE")

	return nil
}

func serverPing() (string, error) {
	respMisc, err := gClient.GetMisc(context.Background(), &pb.Misc{Mtype: "ping", Data: []byte(strings.Join([]string{"ping from", Host}, " <= "))})
	if err != nil {
		PrintError("Ping:", err)
		return "", err
	}
	var rt string
	if string(respMisc.Data) == "pong" {
		rt = "pong"
	}

	if string(respMisc.Data) == "error" {
		rt = "error"
	}
	return rt, nil
}

func SetClientStreamConn() (err error) {
	hostPort := strings.Join([]string{Host, Port}, ":")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gClientConn, err = grpc.DialContext(ctx, hostPort,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize), grpc.MaxCallSendMsgSize(MaxMessageSize)))

	if err != nil {
		PrintError("ClientSendFiles", err)
		return err
	}

	//defer gClientConn.Close()

	gClient = pb.NewFileTransferClient(gClientConn)
	if gClient == nil {
		return NewError("gClient cannot be empty")
	}

	gClientStream, err = gClient.StreamReceive(context.Background())
	if err != nil {
		PrintError("ClientSendFiles", err)
		return err
	}

	return nil
}

func SetTLSClientStreamConn() (err error) {
	certificate, err := tls.LoadX509KeyPair("cert/client/client.crt", "cert/client/client.key")
	if err != nil {
		FatalError("SetTLSClientStreamConn:tls.LoadX509KeyPair", err)
	}

	certPool := x509.NewCertPool()
	ca, err := os.ReadFile("cert/ca.crt")
	if err != nil {
		FatalError("SetTLSClientStreamConn:os.ReadFile", err)
	}

	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		FatalError("SetTLSClientStreamConn:os.ReadFile", NewError("certPool.AppendCertsFromPEM"))
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(
			&tls.Config{
				ServerName:   Host,
				Certificates: []tls.Certificate{certificate},
				RootCAs:      certPool,
			})),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize), grpc.MaxCallSendMsgSize(MaxMessageSize)),
	}

	hostPort := strings.Join([]string{Host, Port}, ":")

	gClientConn, err = grpc.Dial(hostPort, opts...)

	if err != nil {
		FatalError("SetTLSClientStreamConn", err)
	}

	//defer gClientConn.Close()

	gClient = pb.NewFileTransferClient(gClientConn)
	if gClient == nil {
		return NewError("gClient cannot be empty")
	}

	gClientStream, err = gClient.StreamReceive(context.Background())
	if err != nil {
		PrintError("ClientSendFiles", err)
		return err
	}

	return nil
}

// ClientSendDirSymlink is for directory and symlink
// run after ClientSendFiles
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

func ClientSetDirSymList() error {
	SourceDir = ToUnixSlash(SourceDir)
	filepath.Walk(SourceDir, func(fpath string, finfo fs.FileInfo, err error) error {
		if err != nil {
			PrintError("ClientSendFiles: filepath.Walk", err)
			return err
		}
		fpath = ToUnixSlash(fpath)

		if fpath == "." || fpath == ".." || fpath == "" {
			return nil
		}

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

		return nil
	})
	//sendFinishSignal(gClientStream)

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

func sendReportSignal(client pb.FileTransferClient) error {
	DebugInfo("sendReportSignal", "Getting report ...")
	respMisc, err := client.GetMisc(context.Background(), &pb.Misc{Mtype: "pbSaveStatus"})
	if err != nil {
		PrintError("sendReportSignal:stream.Recv", err)
		return err
	}

	successTxt := filepath.Join(LogDir, GetNowTimeStr("Ymd"), strings.Join([]string{GetNowTimeStr("H"), "rpcopy", "success.log"}, "_"))
	failureTxt := filepath.Join(LogDir, GetNowTimeStr("Ymd"), strings.Join([]string{GetNowTimeStr("H"), "rpcopy", "error.log"}, "_"))
	MakeDirs(filepath.Dir(failureTxt))

	successWriter, err := os.OpenFile(successTxt, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	PrintError("sendReportSignal:os.OpenFile", err)
	failureWriter, err := os.OpenFile(failureTxt, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	PrintError("sendReportSignal:os.OpenFile", err)

	if err != nil {
		return err
	}

	var m map[string]int64
	if respMisc.Data != nil {
		m2, err := Byte2Map(respMisc.Data, m)
		PrintError("sendReportSignal:Byte2Map", err)
		line := ""
		for k, v := range m2 {
			line = fmt.Sprintf("%d, %s \n", v, k)
			if v > 199 && v < 400 {
				successWriter.WriteString(line)
			} else {
				failureWriter.WriteString(line)
			}
		}
	}

	successWriter.Close()
	failureWriter.Close()

	DebugInfo("sendReportSignal", "Done.")

	return nil
}

func SetFileList() error {
	DebugInfo("SetFileList", "Getting targetFileList ...")
	respMisc, err := gClient.GetMisc(context.Background(), &pb.Misc{Mtype: "targetFileList"})
	if err != nil {
		PrintError("SetFileList:client.GetMisc", err)
		return err
	}
	var tfl map[string]int64
	var targetFileList map[string]int64
	if respMisc != nil {
		targetFileList, err = Byte2Map(respMisc.Data, tfl)
		if err != nil {
			PrintError("SetFileList: Byte2Map", err)
			return err
		}
	}

	sourceFileList := GetFileList(SourceDir, true)
	DebugInfo("SetFileList:targetFileList", len(targetFileList))
	DebugInfo("SetFileList:sourceFileList", len(sourceFileList))

	sendFileList = make(map[string]int64, len(sourceFileList)-len(targetFileList))
	for spath, size := range sourceFileList {
		if _, ok := targetFileList[spath]; !ok {
			sendFileList[spath] = size
		}
	}

	DebugInfo("SetFileList:sendFileList", len(sendFileList))
	for spath, size := range sendFileList {
		if size < BoltSplitSize {
			smallFileList = append(smallFileList, spath)
		} else {
			largeFileList = append(largeFileList, spath)
		}
	}
	DebugInfo("SetFileList:smallFileList", len(smallFileList))
	DebugInfo("SetFileList:largeFileList", len(largeFileList))

	fdump := filepath.Join(LogDir, GetNowTimeStr("Ymd"), strings.Join([]string{GetNowTimeStr("H"), "rpcopy", "send_list.txt"}, "_"))
	MakeDirs(filepath.Dir(fdump))
	dumpWriter, err := os.OpenFile(fdump, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		PrintError("SetFileList: Dump sendFileList", err)
		return err
	}

	var sflKeys []string

	for k := range sendFileList {
		sflKeys = append(sflKeys, k)
	}

	sort.Strings(sflKeys)

	for _, k := range sflKeys {
		line := fmt.Sprintf("%s\n", k)
		dumpWriter.WriteString(line)
	}
	dumpWriter.Close()

	return nil
}

func ClientSendLargeFileList() error {
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
		err = pbFileChunkSend(fpath, pbFile, gClientStream)
		PrintError("ClientSendLargeFileList: pbFileChunkSend", err)
	}
	return nil
}

func ClientSendSmallFileList() error {
	var boltFileList []string
	var bsize int64 = 0
	var countBoltFileList int

	for _, spath := range smallFileList {
		bsize += sendFileList[spath]
		if bsize < MaxBoltSize {
			boltFileList = append(boltFileList, spath)
		}

		if bsize >= MaxBoltSize {
			DebugInfo("ClientSendFiles: bsize", bsize>>20, " MB, MaxBoltSize= ", MaxBoltSize>>20, " MB")
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
