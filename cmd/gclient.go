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

func gClientIsSame(relpathsize map[string]string, clientMisc pb.FileTransferClient) map[string]bool {
	result := make(map[string]bool, len(relpathsize))
	m := map[string]any{}
	for relpath, size := range relpathsize {
		result[relpath] = false
		m[relpath] = size
	}

	if IsWithDiff == false {
		return result
	}

	b, err := Map2Bytes(m)
	if err != nil {
		PrintError("gClientIsSame", err)
	}

	misc := pb.Misc{
		Type: "difflist",
		Data: b,
	}

	resp, err := clientMisc.SyncMisc(context.Background(), &misc)
	if err != nil {
		PrintError("gClientIsSame", err)
		return result
	}

	mdst, err := Bytes2MapString(resp.Data)
	if err != nil {
		PrintError("gClientIsSame", err)
		return result
	}
	for relpath, v := range mdst {
		if v == "-1" {
			result[relpath] = false
			continue
		}
		if v == "0" {
			result[relpath] = true
			continue
		}
		if v == hashFile(ToUnixSlash(filepath.Join(SourceDir, relpath))) {
			result[relpath] = true
			continue
		}
		result[relpath] = false
	}

	return result
}

func gClientSyncFolderSymlink() error {
	DebugInfo("gClientSyncFolderSymlink", "...")
	client := GetClient()
	lenSym := len(symLinkMap)
	if lenSym > 0 {
		atomic.AddInt32(&totalNum, int32(lenSym))
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

	var relFpath string
	var batchFiles map[string]string = make(map[string]string, 2000)
	SourceDir = ToUnixSlash(SourceDir)
	filepath.Walk(SourceDir, func(fpath string, finfo fs.FileInfo, err error) error {
		if err != nil {
			PrintError("selectFiles", err)
		}

		fpath = ToUnixSlash(fpath)
		relFpath = strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/")

		if finfo.IsDir() {
			atomic.AddInt32(&selectFolder, 1)
			folderInfoMap[relFpath] = NewFinfoLite(finfo.Size(), finfo.ModTime(), finfo.Mode())
			return nil
		}

		if IsFollowSymlink == false {
			if IsSymlink(fpath) {
				targetFile := ToUnixSlash(GetSymlink(fpath))
				symLinkMap[relFpath] = targetFile
				atomic.AddInt32(&selectSymlink, 1)
				return nil
			}
		}

		if IsFileNeeded(fpath, finfo) == false {
			return nil
		}

		batchFiles[relFpath] = Int64Str(finfo.Size())
		if len(batchFiles) > 2000 {
			files2chan(batchFiles)
			batchFiles = make(map[string]string, 2000)
		}

		return nil
	})

	if len(batchFiles) > 0 {
		files2chan(batchFiles)
	}
	chanLargeFiles <- AllDone
	chanSmallFiles <- AllDone

	PrintlnInfo("purple", "Task Count",
		"Large: ", atomic.LoadInt32(&selectLarge),
		", Small: ", atomic.LoadInt32(&selectSmall),
		", Folder: ", atomic.LoadInt32(&selectFolder),
		", Symlink: ", atomic.LoadInt32(&selectSymlink))

	return nil
}

func files2chan(roundFiles map[string]string) error {
	client := GetClient()
	var fsize int64
	relpathbool := gClientIsSame(roundFiles, client)
	for rpath, bl := range relpathbool {
		if bl == true {
			continue
		}
		//
		srcPath := ToUnixSlash(filepath.Join(SourceDir, rpath))
		fsize = GetFileSize(srcPath)
		if fsize == -1 {
			continue
		}
		if fsize > largeSmallThreshold {
			chanLargeFiles <- srcPath
			atomic.AddInt32(&selectLarge, 1)
		} else {
			chanSmallFiles <- srcPath
			atomic.AddInt32(&selectSmall, 1)
		}
		atomic.AddInt64(&totalSize, fsize)
		atomic.AddInt32(&totalNum, 1)
		PrintSpinner(Int32Str(atomic.LoadInt32(&totalNum)))
	}

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

	client := GetClient()
	SourceDir = ToUnixSlash(SourceDir)
	var relFpath string
	var batchFiles map[string]string = make(map[string]string, 2000)
	filepath.Walk(SourceDir, func(fpath string, finfo fs.FileInfo, err error) error {
		if err != nil {
			PrintError("selectFiles", err)
		}

		fpath = ToUnixSlash(fpath)
		relFpath = strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/")
		if finfo.IsDir() {
			return nil
		}

		batchFiles[relFpath] = Int64Str(finfo.Size())
		if len(batchFiles) > 2000 {
			roundFiles := batchFiles
			relpathbool := gClientIsSame(roundFiles, client)
			for rpath, bl := range relpathbool {
				if bl == false {
					fmt.Println(rpath)
					atomic.AddInt32(&nDiff, 1)
				} else {
					atomic.AddInt32(&nSame, 1)
				}
			}
			batchFiles = make(map[string]string, 2000)
		}

		return nil
	})

	relpathbool := gClientIsSame(batchFiles, client)
	for rpath, bl := range relpathbool {
		if bl == false {
			fmt.Println(rpath)
			atomic.AddInt32(&nDiff, 1)
		} else {
			atomic.AddInt32(&nSame, 1)
		}
	}
	fmt.Println(Cyan("-----------------------------------------"))
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
		DebugInfo("file2pbFile", "path should not be a directory: ", fpath)
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
