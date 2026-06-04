package cmd

import (
	"path/filepath"
	"sync"
	"sync/atomic"
)

func taskSendLargeFiles() error {
	var taskCount int32 = 0
	wg := sync.WaitGroup{}
	for {
		ch := <-chanLargeFile
		if ch == "_ALLDONE_" {
			break
		}

		//DebugInfo("taskSendLargeFiles", ch)
		wg.Add(1)
		go func(relPath string) error {
			defer wg.Done()
			srcPath := ToUnixSlash(filepath.Join(SourceDir, relPath))
			if !Exists(srcPath) {
				PrintError("taskSendLargeFiles", NewError("file does not exist: ", relPath))
				return nil
			}
			pbHeadFile := NewPbFile()
			pbHeadFile.Ftype = []byte("file")
			pbHeadFile.Data = nil
			pbHeadFile.Path = []byte(ToUnixSlash(relPath))
			dstStatus, dstHash := gClientHead(&pbHeadFile)

			isDifferent := false
			srcHash := ""

			if dstStatus == 404 {
				isDifferent = true
			}

			if dstStatus == 200 {
				srcHash = hashFile(srcPath)
				if dstHash != srcHash {
					isDifferent = true
				}
			}

			DebugInfo("taskSendLargeFiles: Head", dstStatus,
				", Different: ", isDifferent,
				" <= server: ", dstHash, " <= client: ", srcHash)

			if !isDifferent {
				DebugInfo("taskSendLargeFiles: SKIP", relPath)
				return nil
			}

			atomic.AddInt32(&totalNum, 1)
			atomic.AddInt64(&totalWriteSize, GetFileSize(srcPath))

			atomic.AddInt32(&taskCount, 1)
			err := gClientStreamSend(srcPath, file2pbFile(srcPath, "file"))
			if err != nil {
				safePbSaveStatus.Store(relPath, "500")
			} else {
				safePbSaveStatus.Store(relPath, "0")
			}
			atomic.AddInt32(&taskCount, -1)

			return nil
		}(ch)

		curTaskCount := atomic.LoadInt32(&taskCount)
		if curTaskCount%8 == 0 {
			wg.Wait()
		}

	}

	wg.Wait()

	return nil
}
