package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	pb "pb"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func gClientHandshake() string {
	cpbf := NewPbFile()
	cpbf.Comment = HealthCheck
	t1 := GetNowTime()
	resp, err := GetClient().Head(context.Background(), &cpbf)
	if err != nil {
		FatalError("cannot connect to server", err)
	}
	if resp.Action != 200 {
		FatalError("cannot connect to server", err)
	}
	if IsWithTLS {
		PrintlnInfo("green", "Server Status", "Connected. Latency: ", time.Since(t1))
	} else {
		PrintlnInfo("purple", "Server Status", "Connected. Latency: ", time.Since(t1))
	}
	return resp.Comment
}

func gClientIsSame(fpath string, clientHead pb.FileTransferClient) bool {
	if IsWithDiff == false {
		return false
	}

	cpbf := file2pbFile(fpath, false)
	resp, err := clientHead.Head(context.Background(), &cpbf)
	if err != nil {
		PrintError("gClientIsSame", err)
		return false
	}
	if resp.Action == -1 {
		DebugInfo("gClientIsSame", resp.Comment)
		return false
	}

	if resp.Action == 0 && resp.Fhash != "" {
		DebugInfo("gClientIsSame", "checking hash => ", filepath.Base(fpath))
		clientHash := hashFile(fpath)
		if resp.Fhash == clientHash {
			return true
		}
		return false
	}

	if resp.Action == 1 {
		return true
	}
	return false
}

func gClientSyncFolderSymlink() error {
	DebugInfo("gClientSyncFolderSymlink", "...")
	client := GetClient()
	// lenSym := len(symLinkMap)
	// if lenSym > 0 {
	// 	PrintlnInfo("cyan", "Symlinks", lenSym)
	// 	atomic.AddInt32(&totalNum, int32(lenSym))
	// 	b, err := Map2Bytes(symLinkMap)
	// 	if err == nil {
	// 		pbm := pb.Misc{Type: "symlink", Data: b}
	// 		client.SyncMisc(context.Background(), &pbm)
	// 	}

	// }

	if len(folderInfoMap) > 0 {
		//PrintlnInfo("cyan", "Folders", len(folderInfoMap))
		b, err := Map2Bytes(folderInfoMap)
		if err == nil {
			pbm := pb.Misc{Type: "folder", Data: b}
			client.SyncMisc(context.Background(), &pbm)
		}
	}

	return nil
}

func selectFiles() error {
	_, err := os.Stat(SourceDir)
	if err != nil {
		PrintError("selectFiles", err)
		return err
	}

	client := GetClient()

	// idx := 0
	var relFpath string
	var fsize int64
	var nLarge, nSmall, nSymlink int32
	SourceDir = ToUnixSlash(SourceDir)
	filepath.Walk(SourceDir, func(fpath string, finfo fs.FileInfo, err error) error {
		if err != nil {
			PrintError("selectFiles", err)
		}

		fpath = ToUnixSlash(fpath)
		relFpath = strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/")

		if finfo.IsDir() {
			folderInfoMap[relFpath] = NewFinfoLite(finfo.Size(), finfo.ModTime(), finfo.Mode())
			return nil
		}

		if IsFollowSymlink == false {
			if IsSymlink(fpath) {
				targetFile := ToUnixSlash(GetSymlink(fpath))
				symLinkMap[relFpath] = targetFile
				atomic.AddInt32(&nSymlink, 1)
				return nil
			}
		}

		if IsFileNeeded(fpath, finfo) == false {
			return nil
		}

		if gClientIsSame(fpath, client) == true {
			return nil
		}

		fsize = finfo.Size()
		if fsize > largeSmallThreshold {
			chanLargeFiles <- fpath
			atomic.AddInt32(&nLarge, 1)
		} else {
			chanSmallFiles <- fpath
			atomic.AddInt32(&nSmall, 1)
		}
		atomic.AddInt64(&totalSize, fsize)
		atomic.AddInt32(&totalNum, 1)
		PrintSpinner(Int32Str(atomic.LoadInt32(&totalNum)))
		return nil
	})

	chanLargeFiles <- AllDone
	chanSmallFiles <- AllDone

	PrintlnInfo("purple", "Task Count",
		"Large: ", atomic.LoadInt32(&nLarge),
		", Small: ", atomic.LoadInt32(&nSmall),
		", Symlink: ", atomic.LoadInt32(&nSymlink))

	return nil
}

func diffFiles() error {
	_, err := os.Stat(SourceDir)
	if err != nil {
		PrintError("selectFiles", err)
		return err
	}
	var nDiff int32
	var nSame int32
	var sem chan struct{} = make(chan struct{}, 4)
	wg := sync.WaitGroup{}
	clients := []pb.FileTransferClient{
		GetClient(),
		GetClient(),
		GetClient(),
		GetClient(),
	}
	idx := 0
	SourceDir = ToUnixSlash(SourceDir)
	filepath.Walk(SourceDir, func(fpath string, finfo fs.FileInfo, err error) error {
		if err != nil {
			PrintError("selectFiles", err)
		}

		fpath = ToUnixSlash(fpath)
		if finfo.IsDir() {
			return nil
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(clientHead pb.FileTransferClient) error {
			defer func() {
				<-sem
				wg.Done()
			}()
			if gClientIsSame(fpath, clientHead) == false {
				atomic.AddInt32(&nDiff, 1)
				PrintlnInfo("yellow", "[DIFF]", strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/"))
				return nil
			} else {
				atomic.AddInt32(&nSame, 1)
			}
			return nil
		}(clients[idx])

		idx++
		if idx > 3 {
			idx = 0
		}

		return nil
	})

	wg.Wait()
	close(sem)

	PrintlnInfo("purple", "Different Files", atomic.LoadInt32(&nDiff))
	PrintlnInfo("white", "Same Files", atomic.LoadInt32(&nSame))

	return nil
}

func NewPbFile() pb.File {
	return pb.File{}
}

func file2pbFile(fpath string, withHash bool) pb.File {
	fpath = ToUnixSlash(fpath)
	pbFile := pb.File{Fpath: ""}
	finfo, err := os.Stat(fpath)
	if err != nil {
		PrintError("file2pbFile", err)
		return pbFile
	}
	if finfo.IsDir() {
		PrintError("file2pbFile", NewError("path is a directory:", fpath))
		return pbFile
	}
	//
	pbFile.Action = 0
	pbFile.Comment = ""
	pbFile.Fpath = strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/")
	pbFile.Fhash = ""
	if withHash {
		pbFile.Fhash = hashFile(fpath)
	}
	pbFile.Fsize = finfo.Size()
	pbFile.Finfo = fileInfo2Bytes(finfo)

	chunkTotal := int32(math.Ceil(float64(finfo.Size()) / float64(chunkSize)))

	pbFile.ChunkNum = 0
	pbFile.ChunkTotal = chunkTotal
	pbFile.ChunkOffset = 0
	pbFile.ChunkHash = ""
	pbFile.ChunkSize = 0
	pbFile.ChunkData = nil
	//
	return pbFile
}

func logSendFailure() error {
	fp, err := os.OpenFile(ToUnixSlash(filepath.Join(LogDir, "send_errors.log")),
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	FatalError("logSendFailure", err)

	sendFailure.Range(func(key any, val any) bool {
		WriteFile(fp, fmt.Appendf([]byte(""), "%s, %s\n", key.(string), val.(string)))
		return true
	})
	fp.Close()

	return nil
}

func gClientGetSpeed() int64 {
	sz := atomic.LoadInt64(&totalSize)
	ts := time.Since(tStart).Seconds()
	speed := int64(float64(sz) / ts)
	return speed
}
