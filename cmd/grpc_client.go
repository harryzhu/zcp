package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	pb "pb"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func pbHeadSourceFiles() error {
	t1 := GetNowTime()
	var headCount int = 0
	var createCount int = 0
	var updateCount int = 0
	var fileHash map[string]string = make(map[string]string, 256)

	SourceDir = ToUnixSlash(SourceDir)
	filepath.WalkDir(SourceDir, func(fpath string, dirInfo fs.DirEntry, err error) error {
		if err != nil {
			PrintError("pbHeadSourceFiles", err)
			return err
		}
		fpath = ToUnixSlash(fpath)

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

		if dirInfo.IsDir() {
			dirList[fpath], err = dirInfo.Info()
			PrintError("pbHeadSourceFiles: dirInfo.Info", err)
			return nil
		}

		if isCopyNeeded(fpath, dirInfo) == false {
			return nil
		}

		relPath := strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/")
		fileHash[relPath] = ""
		headCount++

		return nil
	})

	pbFile := NewPbFile()
	pbFile.Ftype = []byte("FileHashList")
	pbFile.Data = ZstdBytes(MapStr2Byte(fileHash))
	pbFile.Fsum = nil
	resp, err := gClient.Head(context.Background(), &pbFile)

	PrintlnInfo("green", "FileHashList Size: ", len(pbFile.Data))
	if err != nil {
		PrintError("pbHeadSourceFiles: gClient.Head", err)
		return err
	}

	var dhl map[string]string
	var diffHashList map[string]string
	if resp.Data != nil {
		respData, err := UnZstdBytes(resp.Data)
		if err != nil {
			PrintError("pbHeadSourceFiles: UnZstdBytes", err)
			return err
		}

		diffHashList, err = Byte2MapStr(respData, dhl)
		if err != nil {
			PrintError("pbHeadSourceFiles: Byte2MapStr", err)
			return err
		}
	}

	for spath, shash := range diffHashList {
		//DebugInfo("pbHeadSourceFiles", spath, " : ", shash)
		fpath := filepath.Join(SourceDir, spath)
		finfo, err := os.Stat(fpath)
		if err != nil {
			PrintError("pbHeadSourceFiles: os.Stat", err)
			continue
		}
		fsize := finfo.Size()
		sendFileList[spath] = fsize
		if shash == "404" {
			if fsize < smallFileSize {
				smallFileList = append(smallFileList, spath)
			} else {
				largeFileList = append(largeFileList, spath)
			}
			createCount++
			continue
		}

		if shash != hashFile(fpath) {
			fsize := finfo.Size()
			if fsize < smallFileSize {
				smallFileList = append(smallFileList, spath)
			} else {
				largeFileList = append(largeFileList, spath)
			}
			updateCount++
			continue
		}

	}

	DebugInfo("pbHeadSourceFiles: Duration", time.Since(t1), ", headCount: ", headCount)
	DebugInfo("pbHeadSourceFiles: createCount", createCount, ", updateCount: ", updateCount)
	DebugInfo("pbHeadSourceFiles: TotalCount", len(sendFileList))
	PrintlnInfo("green", "--------------------------------------", "")
	PrintlnInfo("green", "pbHeadSourceFiles: smallFileList", len(smallFileList))
	PrintlnInfo("green", "pbHeadSourceFiles: largeFileList", len(largeFileList))
	PrintlnInfo("green", "pbHeadSourceFiles: symlinkList", len(symList))
	PrintlnInfo("green", "--------------------------------------", "")

	sort.Strings(smallFileList)
	sort.Strings(largeFileList)
	dumpFileList(smallFileList, "rpcopy_send_smallFileList")
	dumpFileList(largeFileList, "rpcopy_send_largeFileList")
	//
	var dirs []string
	var syms []string
	for d, _ := range dirList {
		dirs = append(dirs, d)
	}
	for d, t := range symList {
		syms = append(syms, strings.Join([]string{d, t}, " -> "))
	}
	sort.Strings(dirs)
	sort.Strings(syms)
	dumpFileList(dirs, "rpcopy_send_dirList")
	dumpFileList(syms, "rpcopy_send_symlinkList")

	return nil
}

func ClientSendSmallFileList() error {
	var zstdFileList []string
	var bsize int64 = 0
	var countZstdFileList int

	for _, spath := range smallFileList {
		bsize += sendFileList[spath]
		if bsize < maxZipSize {
			zstdFileList = append(zstdFileList, spath)
		}

		if bsize >= maxZipSize {
			DebugInfo("ClientSendSmallFileList: bsize", bsize>>20, " MB, MaxBoltSize= ", maxZipSize>>20, " MB")
			countZstdFileList = len(zstdFileList)
			atomic.AddInt32(&totalNum, int32(countZstdFileList))
			atomic.AddInt64(&totalWriteSize, bsize)
			err := createZip(zstdFileList, strings.Join([]string{"rpcopy_client.db", "bolt"}, "_"))
			PrintError("ClientSendSmallFileList:createZip", err)
			zstdFileList = zstdFileList[:0]
			bsize = 0
		}
	}

	if len(zstdFileList) > 0 {
		countZstdFileList = len(zstdFileList)
		err := createZip(zstdFileList, strings.Join([]string{"rpcopy_client.db", "bolt"}, "_"))
		atomic.AddInt32(&totalNum, int32(countZstdFileList))
		atomic.AddInt64(&totalWriteSize, bsize)
		PrintError("ClientSendSmallFileList:createZip", err)
	}
	//

	return nil
}

func ClientSendLargeFileList() error {
	wg := sync.WaitGroup{}
	var count int32 = 0
	for _, spath := range largeFileList {
		fpath := ToUnixSlash(filepath.Join(SourceDir, spath))
		finfo, err := os.Stat(fpath)
		if err != nil {
			PrintError("ClientSendLargeFileList", err)
			continue
		}

		wg.Add(1)
		go func(fpath string, finfo fs.FileInfo) error {
			defer wg.Done()
			//PrintlnInfo("white", "Sending", fpath)
			pbFile := file2pbFile(fpath, finfo, "file")

			atomic.AddInt32(&totalNum, 1)
			atomic.AddInt64(&totalWriteSize, finfo.Size())
			DebugInfo("ClientSendLargeFileList: Sending", fpath)

			atomic.AddInt32(&count, 1)
			err = pbFileChunkSend(fpath, pbFile)
			atomic.AddInt32(&count, -1)
			PrintError("ClientSendLargeFileList: pbFileChunkSend", err)
			return nil
		}(fpath, finfo)

		atomic.AddInt32(&count, 1)

		if atomic.LoadInt32(&count) > int32(2) {
			wg.Wait()
		}

	}
	wg.Wait()

	return nil
}

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

func ClientGetReport() error {
	DebugInfo("ClientGetReport", "Getting report ...")
	respMisc, err := gClient.GetMisc(context.Background(), &pb.Misc{Mtype: "pbSaveStatus"})
	if err != nil {
		PrintError("ClientGetReport:stream.Recv", err)
		return err
	}

	successTxt := filepath.Join(LogDir, GetNowTimeStr("Ymd"), strings.Join([]string{GetNowTimeStr("H"), "rpcopy", "success.log"}, "_"))
	failureTxt := filepath.Join(LogDir, GetNowTimeStr("Ymd"), strings.Join([]string{GetNowTimeStr("H"), "rpcopy", "error.log"}, "_"))
	MakeDirs(filepath.Dir(failureTxt))

	successWriter, err := os.OpenFile(successTxt, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	PrintError("ClientGetReport:os.OpenFile", err)
	failureWriter, err := os.OpenFile(failureTxt, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	PrintError("ClientGetReport:os.OpenFile", err)

	if err != nil {
		return err
	}

	var m map[string]string
	if respMisc.Data != nil {
		m2, err := Byte2MapStr(respMisc.Data, m)
		PrintError("ClientGetReport:Byte2MapStr", err)
		line := ""
		for k, v := range m2 {
			line = fmt.Sprintf("%s, %s \n", v, k)
			vint, err := strconv.Atoi(v)
			PrintError("ClientGetReport: strconv.Atoi", err)
			if vint > 199 && vint < 400 {
				successWriter.WriteString(line)
			} else {
				failureWriter.WriteString(line)
			}
		}
	}

	successWriter.Close()
	failureWriter.Close()

	DebugInfo("ClientGetReport", "Done.")

	return nil
}

func ClientSendAllFiles() error {
	pbHeadFile := NewPbFile()
	pbHeadFile.Ftype = []byte("file")
	pbHeadFile.Data = nil
	pbHeadFile.Fsum = nil

	wg := sync.WaitGroup{}
	var count int32 = 0

	SourceDir = ToUnixSlash(SourceDir)
	filepath.WalkDir(SourceDir, func(fpath string, dirInfo fs.DirEntry, err error) error {
		if err != nil {
			PrintError("ClientSendAllFiles", err)
			return err
		}
		fpath = ToUnixSlash(fpath)

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

		if dirInfo.IsDir() {
			dirList[fpath], err = dirInfo.Info()
			PrintError("pbHeadSourceFiles: dirInfo.Info", err)

			return nil
		}

		atomic.AddInt32(&totalNum, 1)
		if isCopyNeeded(fpath, dirInfo) == false {
			return nil
		}

		relPath := strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/")
		pbHeadFile.Path = []byte(relPath)
		resp, err := gClient.Head(context.Background(), &pbHeadFile)

		if err != nil {
			PrintError("ClientSendAllFiles: gClient.Head", err)
			return err
		}

		isSend := false

		if resp.Status == 404 {
			isSend = true
		}

		if resp.Status == 200 {
			srcHash := hashFile(fpath)
			dstHash := string(resp.Fsum)
			if srcHash != dstHash {
				isSend = true
			}
		}

		if isSend {
			finfo, err := dirInfo.Info()
			if err != nil {
				PrintError("ClientPsendAllFiles: dirInfo.Info", err)
				return err
			}

			wg.Add(1)
			go func(fpath string, finfo fs.FileInfo) error {
				defer wg.Done()
				//PrintlnInfo("white", "Sending", fpath)
				pbFile := file2pbFile(fpath, finfo, "file")

				atomic.AddInt64(&totalWriteSize, finfo.Size())
				DebugInfo("ClientPsendAllFiles: Sending", fpath)

				atomic.AddInt32(&count, 1)
				err = pbFileChunkSend(fpath, pbFile)
				atomic.AddInt32(&count, -1)
				PrintError("ClientPsendAllFiles: pbFileChunkSend", err)
				return nil
			}(fpath, finfo)

			atomic.AddInt32(&count, 1)
		}

		if atomic.LoadInt32(&count) > int32(8) {
			wg.Wait()
		}

		return nil
	})

	wg.Wait()

	return nil
}

func ClientDiffAllFiles() error {
	var fileHash map[string]string = make(map[string]string, 256)
	var diffCount int32 = 0

	SourceDir = ToUnixSlash(SourceDir)
	filepath.WalkDir(SourceDir, func(fpath string, dirInfo fs.DirEntry, err error) error {
		if err != nil {
			PrintError("ClientDiffAllFiles", err)
			return err
		}
		fpath = ToUnixSlash(fpath)

		if dirInfo.IsDir() {
			return nil
		}

		atomic.AddInt32(&totalNum, 1)

		relPath := strings.TrimPrefix(strings.TrimPrefix(fpath, SourceDir), "/")
		fileHash[relPath] = ""

		return nil
	})

	pbHeadFile := NewPbFile()
	pbHeadFile.Ftype = []byte("FileHashList")
	pbHeadFile.Data = ZstdBytes(MapStr2Byte(fileHash))
	pbHeadFile.Fsum = nil

	resp, err := gClient.Head(context.Background(), &pbHeadFile)

	if err != nil {
		PrintError("pbHeadSourceFiles: gClient.Head", err)
		return err
	}

	PrintlnInfo("green", "pbHeadSourceFiles: FileHashList Size: ", len(pbHeadFile.Data))

	var dhl map[string]string
	var diffHashList map[string]string
	if resp.Data != nil {
		respData, err := UnZstdBytes(resp.Data)
		if err != nil {
			PrintError("pbHeadSourceFiles: UnZstdBytes", err)
			return err
		}

		diffHashList, err = Byte2MapStr(respData, dhl)
		if err != nil {
			PrintError("pbHeadSourceFiles: Byte2MapStr", err)
			return err
		}
	}

	var diffFiles []string
	for spath, shash := range diffHashList {
		fpath := filepath.Join(SourceDir, spath)
		_, err := os.Stat(fpath)
		if err != nil {
			PrintError("pbHeadSourceFiles: os.Stat", err)
			continue
		}
		if shash == "404" {
			diffFiles = append(diffFiles, spath)
			continue
		}

		if shash != hashFile(fpath) {
			diffFiles = append(diffFiles, spath)
			continue
		}

	}

	sort.Strings(diffFiles)

	for _, spath := range diffFiles {
		fmt.Printf("%s\n", spath)
		diffCount++
	}

	fmt.Printf("Total: %d , Different: %d\n", atomic.LoadInt32(&totalNum), diffCount)
	dumpFileList(diffFiles, "rpcopy_diff_DifferentFiles")

	return nil
}
