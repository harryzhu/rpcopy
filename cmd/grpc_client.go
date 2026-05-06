package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	pb "pb"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
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

	var chanChunkSendStatus chan int = make(chan int, 1)
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func(stream pb.FileTransfer_StreamReceiveClient) error {
		defer wg.Done()
		for {
			cs := <-chanChunkSendStatus
			if cs == -1 {
				break
			}
			resp, err := stream.Recv()
			if err != nil {
				PrintError("pbFileChunkSend:stream.Recv", err)
				chanChunkSendStatus <- -1
			}

			DebugInfo("pbFileChunkSend:stream.Recv", resp.Status)
		}
		return nil
	}(stream)

	go func(fpath string, pbFile *pb.File, stream pb.FileTransfer_StreamReceiveClient) error {
		defer wg.Done()

		fp, err := os.Open(fpath)
		if err != nil {
			PrintError("pbFileChunkSend:os.Open", err)
			chanChunkSendStatus <- -1
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
				chanChunkSendStatus <- -1
				break
			}

			if n == 0 || err == io.EOF {
				chanChunkSendStatus <- -1
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
				chanChunkSendStatus <- -1
				break
			}

			DebugInfo("pbFileChunkSend: ChanNum", chunkNum, " / ", chunkTotal, " : ", n)
			chunkNum++

		}
		// DebugInfo("pbFileChunkSend: ONE_DONE", chunkNum, " / ", chunkTotal, "=>", channum,
		// 	" ::", fpath, " ==> Chunks=", pbFile.Chunks, " <= Size:", pbFile.Fsize, " <= ", float64(pbFile.Fsize)/float64(chunkSize))
		// //
		chanChunkSendStatus <- -1
		return nil
	}(fpath, pbFile, stream)

	wg.Wait()

	DebugInfo("pbFileChunkSend", "ONE_DONE")

	return nil
}

func pbFileHead(pbFile *pb.File, clientHead pb.FileTransferClient) *pb.File {
	resp, err := clientHead.Head(context.Background(), pbFile)
	PrintError("pbFileHead", err)
	return resp
}

func GetClientStreamConn() (client pb.FileTransferClient, clientStream pb.FileTransfer_StreamReceiveClient, conn *grpc.ClientConn, err error) {
	hostPort := strings.Join([]string{Host, Port}, ":")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err = grpc.DialContext(ctx, hostPort, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxMessageSize), grpc.MaxCallSendMsgSize(MaxMessageSize)))
	if err != nil {
		PrintError("ClientSendFiles", err)
		return nil, nil, nil, err
	}
	//defer conn.Close()

	client = pb.NewFileTransferClient(conn)
	if client == nil {
		return nil, nil, nil, err
	}

	clientStream, err = client.StreamReceive(context.Background())
	if err != nil {
		PrintError("ClientSendFiles", err)
		return nil, nil, nil, err
	}

	return client, clientStream, conn, nil
}

func ClientSendFiles() error {
	client, clientStream, conn, err := GetClientStreamConn()
	if err != nil {
		PrintError("ClientSendFiles", err)
		return err
	}
	defer conn.Close()

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
		totalWriteSize += finfo.Size()

		err = pbFileChunkSend(fpath, pbFile, clientStream)
		PrintError("ClientSendFiles:wgSend:pbFileChunkSend", err)
		if totalNum%10 == 0 {
			PrintSpinner2(strings.Join([]string{Int2Str(totalNum), " => "}, ""),
				strings.Join([]string{Int64Str(totalWriteSize >> 20), " MB"}, ""))
		}
		return nil
	})

	//
	for k, v := range dirList {
		pbFile := file2pbFile(k, v, "dir")
		//
		err = clientStream.Send(pbFile)
		if err != nil {
			PrintError("ClientSendFiles", err)
			continue
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
			continue
		}

	}
	//
	sendFinishSignal(clientStream)

	sendReportSignal(client)
	//
	time.Sleep(2 * time.Second)
	DebugWarn("ClientSendFiles: CloseSend", "............")
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

	var m map[string]int
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
