package cmd

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
)

func taskDiffFiles() error {
	var srcFileList map[string][]byte = make(map[string][]byte, 512)
	var srcLargeFile map[string][]byte = make(map[string][]byte, 512)
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		emptyByte := []byte("")
		for {
			ch := <-chanLargeFile
			if ch == "_ALLDONE_" {
				break
			}
			srcLargeFile[ch] = emptyByte
			atomic.AddInt32(&totalNum, 1)
		}
	}()

	go func() {
		defer wg.Done()
		emptyByte := []byte("")
		for {
			ch := <-chanSmallFile
			if ch == "_ALLDONE_" {
				break
			}
			srcFileList[ch] = emptyByte
			atomic.AddInt32(&totalNum, 1)
		}
	}()

	wg.Wait()

	for k, v := range srcLargeFile {
		srcFileList[k] = v
	}

	diffFileList, err := getDifferentFiles(srcFileList)
	if err != nil {
		PrintError("taskDiffFiles", err)
		return err
	}

	if len(diffFileList) > 0 {
		fmt.Println(sepLine)
		for _, dpath := range diffFileList {
			fmt.Println(dpath)
			atomic.AddInt64(&totalWriteSize, GetFileSize(ToUnixSlash(filepath.Join(SourceDir, dpath))))
		}
	}

	fmt.Println(sepLine)
	PrintlnInfo("green", "Files", "Different: ", len(diffFileList))

	resultFile := DumpFileList(diffFileList, "zcp_different_files.txt")
	PrintlnInfo("white", "save results into file", resultFile)
	return nil
}
