package cmd

import (
	"io/fs"
	pb "pb"
	"regexp"
	"sync"

	"google.golang.org/grpc"
)

const (
	maxBoltSize int64 = 1 << 30
	// < 8 MB :use boltdb
	// >= 8 MB use rawfile
	smallFileSize    int64 = 2 << 20
	mediumFileSize64 int64 = 256 << 20
	//
	chunkSize int = 32 << 20
	//
	MaxMessageSize int    = 4 << 30
	SEP            string = "----------------------------------------------------------------"
)

var (
	//
	gClient       pb.FileTransferClient
	gClientStream pb.FileTransfer_StreamReceiveClient
	gClientConn   *grpc.ClientConn
	//
	chanFile  chan *pb.File = make(chan *pb.File, 4)
	chanFile1 chan *pb.File = make(chan *pb.File, 4)
	chanFile2 chan *pb.File = make(chan *pb.File, 4)
	chanFile3 chan *pb.File = make(chan *pb.File, 4)
	//
	safePbSaveStatus sync.Map
	pbSaveStatus     map[string]string = make(map[string]string, 2048)
	progressFlag     int32             = 0
	//
	fextMatch *regexp.Regexp
)

var (
	sendFileList   map[string]int64 = make(map[string]int64, 1024)
	smallFileList  []string
	mediumFileList []string
	largeFileList  []string
	dirList        map[string]fs.FileInfo = make(map[string]fs.FileInfo, 2048)
	symList        map[string]string      = make(map[string]string, 256)
)

var (
	timeGetStart   int64 = 0
	timeGetStop    int64 = 0
	timeDuration   int64 = 0
	totalWriteSize int64 = 0
	totalSpeed     int64 = 0
	totalNum       int32 = 0
	chanNum        int32 = 0
)
