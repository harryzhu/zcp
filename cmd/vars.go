package cmd

import (
	"sync"
)

var (
	chanLargeFiles chan string = make(chan string, 2048)
	chanSmallFiles chan string = make(chan string, 8192)
)

var (
	chunkSize           int64 = 1 << 20
	totalSize           int64
	totalNum            int32
	largeSmallThreshold int64  = 16 << 20
	AllDone             string = "__ALL_DONE__"
	HealthCheck         string = "__HEALTHCHECK__"
)

var (
	sendFailure   sync.Map
	symLinkMap    map[string]any = make(map[string]any, 64)
	folderInfoMap map[string]any = make(map[string]any, 256)
)

var (
	selectFolder  int32
	selectLarge   int32
	selectSmall   int32
	selectSymlink int32
)
