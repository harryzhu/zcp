package cmd

import (
	"sync"
	"sync/atomic"
)

func taskSendLargeFiles() error {
	maxTask := 4
	if IsSerial {
		maxTask = 1
	}
	sem := make(chan struct{}, maxTask)
	wg := sync.WaitGroup{}
	for {
		ch := <-chanLargeFiles
		if ch == AllDone {
			break
		}
		//fmt.Println(ch)
		sem <- struct{}{}
		wg.Add(1)
		go func(ch string) {
			defer func() {
				<-sem
				wg.Done()
			}()
			chunkSend(ch, 100)
			//
			if IsDebug {
				atomic.AddInt64(&totalSize, int64(GetFileSize(ch)))
				atomic.AddInt32(&totalNum, 1)
				PrintSpinner(Int32Str(atomic.LoadInt32(&totalNum)))
			}
		}(ch)
	}
	wg.Wait()
	close(sem)

	return nil
}

func taskSendSmallFiles() error {
	var smallFiles []string = []string{}
	var zipSize int64 = 1 << 30
	var n int64
	var nSum int64
	for {
		ch := <-chanSmallFiles
		if ch == AllDone {
			break
		}
		n = GetFileSize(ch)
		if n != -1 {
			nSum += n
			smallFiles = append(smallFiles, ch)
			if IsDebug {
				atomic.AddInt64(&totalSize, int64(n))
				atomic.AddInt32(&totalNum, 1)
				PrintSpinner(Int32Str(atomic.LoadInt32(&totalNum)))
			}
		}

		if nSum > zipSize && len(smallFiles) > 0 {
			createZip(smallFiles)
			smallFiles = smallFiles[:0]
			nSum = 0
		}

	}
	if len(smallFiles) > 0 {
		createZip(smallFiles)
	}

	return nil
}
