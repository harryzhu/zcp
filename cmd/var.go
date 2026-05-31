package cmd

import (
	"io/fs"
	pb "pb"
	"regexp"
	"sync"

	"google.golang.org/grpc"
)

const (
	maxZipSize int64 = 3 << 30
	// < smallFileSize MB :use zip
	// >= smallFileSize MB use rawfile
	smallFileSize int64 = 32 << 20
	//
	chunkSize int = 1 << 20
	//
	MaxMessageSize int    = 4 << 30
	sepLine        string = "----------------------------------------------------------------"
)

var (
	//
	gClient       pb.FileTransferClient
	gClientStream pb.FileTransfer_StreamReceiveClient
	gClientConn   *grpc.ClientConn
	//
	safePbSaveStatus sync.Map
	pbSaveStatus     map[string]string = make(map[string]string, 256)
	progressFlag     int32             = 0
	//
	fextMatch *regexp.Regexp
)

var (
	sendFileList  map[string]int64 = make(map[string]int64, 256)
	smallFileList []string
	largeFileList []string
	dirList       map[string]fs.FileInfo = make(map[string]fs.FileInfo, 256)
	symList       map[string]string      = make(map[string]string, 32)
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
