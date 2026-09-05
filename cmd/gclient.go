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

func gClientHandshake() {
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
}

func gClientIsSame(fpath string, finfo fs.FileInfo, clientHead pb.FileTransferClient) bool {
	if IsWithDiff == false {
		return false
	}
	if finfo.Size() < ignoreDiffSize {
		return false
	}
	cpbf := file2pbFile(fpath)
	resp, err := clientHead.Head(context.Background(), &cpbf)
	if err != nil {
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
	if len(symLinkMap) > 0 {
		b, err := Map2Bytes(symLinkMap)
		if err == nil {
			pbm := pb.Misc{Type: "symlink", Data: b}
			client.SyncMisc(context.Background(), &pbm)
		}

	}

	if len(folderInfoMap) > 0 {
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
	var sem chan struct{} = make(chan struct{}, 8)
	wg := sync.WaitGroup{}

	clients := []pb.FileTransferClient{
		GetClient(),
		GetClient(),
		GetClient(),
		GetClient(),
		GetClient(),
		GetClient(),
		GetClient(),
		GetClient(),
	}

	idx := 0
	var relFpath string
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
				dstSym := ToUnixSlash(GetSymlink(fpath))
				symLinkMap[relFpath] = dstSym
				return nil
			}
		}

		if IsFileNeeded(fpath, finfo) == false {
			return nil
		}
		sem <- struct{}{}
		wg.Add(1)

		go func(clientHead pb.FileTransferClient) error {
			defer func() {
				<-sem
				wg.Done()
			}()
			if gClientIsSame(fpath, finfo, clientHead) == true {
				DebugInfo("[SKIP]", strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/"))
				return nil
			}

			if finfo.Size() > largeSmallThreshold {
				chanLargeFiles <- fpath
			} else {
				chanSmallFiles <- fpath
			}

			return nil
		}(clients[idx])

		idx++
		if idx > 7 {
			idx = 0
		}

		return nil
	})

	wg.Wait()
	close(sem)

	chanLargeFiles <- AllDone
	chanSmallFiles <- AllDone

	return nil
}

func NewPbFile() pb.File {
	return pb.File{}
}

func file2pbFile(fpath string) pb.File {
	fpath = ToUnixSlash(fpath)
	pbFile := pb.File{}
	finfo, err := os.Stat(fpath)
	if err != nil {
		PrintError("file2pbFile", err)
		return pbFile
	}
	//
	pbFile.Action = 0
	pbFile.Comment = ""
	pbFile.Fpath = strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/")
	pbFile.Fhash = hashFile(fpath)
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
	if len(sendFailure) > 0 {
		fp, err := os.OpenFile(ToUnixSlash(filepath.Join(LogDir, "send_errors.log")),
			os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
		FatalError("logSendFailure", err)
		for k, v := range sendFailure {
			WriteFile(fp, fmt.Appendf([]byte(""), "%s, %s\n", k, v))
		}
		fp.Close()

	}

	return nil
}

func gClientGetSpeed() int64 {
	sz := atomic.LoadInt64(&totalSize)
	ts := time.Since(tStart).Seconds()
	speed := int64(float64(sz) / ts)
	return speed
}
