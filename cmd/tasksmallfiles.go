package cmd

import (
	"os"
	"path/filepath"
	"sort"
)

func getDifferentFiles(flist map[string][]byte) ([]string, error) {
	var diffFileList []string

	batchHeadMisc := NewPbMisc("FileHashList", Map2Byte(flist))
	respData := gClientGetMisc(&batchHeadMisc)
	dstHashList, err := Byte2MapStr(respData)
	if err != nil {
		PrintError("getDifferentFiles: Byte2MapStr", err)
		return diffFileList, err
	}

	for dpath, dhash := range dstHashList {
		srcPath := ToUnixSlash(filepath.Join(SourceDir, dpath))
		if !Exists(srcPath) {
			PrintError("getDifferentFiles: Exists", NewError("file does not exist: ", srcPath))
			continue
		}
		if dhash == "404" {
			DebugInfo("getDifferentFiles: not exist", dpath)
			diffFileList = append(diffFileList, dpath)
			continue
		}
		if dhash != hashFile(srcPath) {
			DebugInfo("getDifferentFiles: different hash", dhash)
			diffFileList = append(diffFileList, dpath)
			continue
		}
	}

	sort.Strings(diffFileList)
	return diffFileList, nil
}

func sendDifferentSmallFiles(diffFileList []string) error {
	if len(diffFileList) > 0 {
		createZip(diffFileList, "zcp_client_DifferentFileList")
	}
	return nil
}

func taskSendSmallFiles() error {
	var srcPath string
	var diffFileList []string
	var err error
	var srcSmallList map[string][]byte = make(map[string][]byte, 256)
	bsize := int64(0)

	for {
		ch := <-chanSmallFile
		if ch == "_ALLDONE_" {
			break
		}

		//DebugInfo("taskSendSmallFiles", ch)
		srcSmallList[ch] = []byte("")
		srcPath = ToUnixSlash(filepath.Join(SourceDir, ch))
		finfo, err := os.Stat(srcPath)
		if err != nil {
			PrintError("taskSendSmallFiles", err)
			return err
		}
		bsize += finfo.Size()

		if bsize > maxZipSize {
			diffFileList, err = getDifferentFiles(srcSmallList)
			if err != nil {
				PrintError("taskSendSmallFiles", err)
				return err
			}
			//
			if len(diffFileList) > 0 {
				PrintlnInfo("green", "taskSendSmallFiles: Batch", len(diffFileList))
				sendDifferentSmallFiles(diffFileList)
			}

			clear(srcSmallList)
			bsize = 0
		}

	}
	//
	if len(srcSmallList) > 0 {
		diffFileList, err = getDifferentFiles(srcSmallList)
		if err != nil {
			PrintError("taskSendSmallFiles", err)
			return err
		}

		if len(diffFileList) > 0 {
			PrintlnInfo("green", "taskSendSmallFiles: Last", bsize>>20, "MB, Files: ", len(diffFileList))
			sendDifferentSmallFiles(diffFileList)
		}

		clear(srcSmallList)
		bsize = 0
	}

	return nil
}
