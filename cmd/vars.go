package cmd

import (
	"io/fs"
	"sync"
)

const (
	maxZipSize int64 = 3 << 30
	//
	maxSmallFileSize int64 = 32 << 20
	//
	chunkSize int = 1 << 20
	//
	maxGrpcMessageSize int = 4 << 30

	sepLine string = "----------------------------------------------------------------"
)

var (
	chanSmallFile chan string = make(chan string, 32768)
	chanLargeFile chan string = make(chan string, 32768)
)

var (
	//
	safePbSaveStatus sync.Map
	pbSaveStatus     map[string][]byte = make(map[string][]byte, 256)
	progressFlag     int32             = 0
	//
	dirList map[string]fs.FileInfo = make(map[string]fs.FileInfo, 256)
	symList map[string]string      = make(map[string]string, 32)
)

var (
	timeStart      int64 = 0
	timeStop       int64 = 0
	timeDuration   int64 = 0
	totalWriteSize int64 = 0
	totalSpeed     int64 = 0
	//
	totalNum int32 = 0
)
