package cmd

import (
	"sync"
	"sync/atomic"
)

func taskSendLargeFiles() error {
	maxTask := 2
	if IsSerial {
		maxTask = 1
	}
	sem := make(chan struct{}, maxTask)
	wg := sync.WaitGroup{}
	for {
		ch := <-chanLargeFiles
		if ch == AllDone {
			PrintlnInfo("purple", "taskLargeFiles", "Done")
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(ch string) {
			defer func() {
				<-sem
				wg.Done()
			}()
			chunkSend(ch, 100)

			if IsDebug {
				PrintSpinner(Int32Str(atomic.LoadInt32(&totalNum)))
			}
		}(ch)
	}
	wg.Wait()
	close(sem)

	return nil
}

func taskSendSmallFiles() error {
	maxTask := 2
	if IsSerial {
		maxTask = 1
	}
	sem := make(chan struct{}, maxTask)

	var smallFiles []string = []string{}
	var zipSize int64 = 512 << 20
	var n int64
	var nSum int64
	var taskId int32
	wg := sync.WaitGroup{}
	for {
		ch := <-chanSmallFiles
		if ch == AllDone {
			PrintlnInfo("purple", "taskSendSmallFiles", "Done")
			break
		}
		n = GetFileSize(ch)
		if n != -1 {
			nSum += n
			smallFiles = append(smallFiles, ch)
		}

		if nSum > zipSize {
			sem <- struct{}{}
			DebugInfo("nSum", nSum>>20, "MB")
			tid := atomic.AddInt32(&taskId, 1)
			batchFiles := smallFiles
			wg.Add(1)
			go func(batchFiles []string, tid int32) {
				defer func() {
					<-sem
					wg.Done()
				}()
				createZip(batchFiles, tid)

			}(batchFiles, tid)
			smallFiles = []string{}
			nSum = 0
		}

	}

	if len(smallFiles) > 0 {
		sem <- struct{}{}
		DebugInfo("nSum", nSum>>20, "MB")
		tid := atomic.AddInt32(&taskId, 1)
		batchFiles := smallFiles
		wg.Add(1)
		go func(batchFiles []string, tid int32) {
			defer func() {
				<-sem
				wg.Done()
			}()
			createZip(smallFiles, tid)
		}(batchFiles, tid)
	}
	wg.Wait()

	return nil
}
