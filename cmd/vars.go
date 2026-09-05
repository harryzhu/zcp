package cmd

var (
	chanLargeFiles chan string = make(chan string, 2048)
	chanSmallFiles chan string = make(chan string, 4096)
)

var (
	chunkSize           int64  = 1 << 20
	totalSize           int64  = 0
	totalNum            int32  = 0
	largeSmallThreshold int64  = 32 << 20
	ignoreDiffSize      int64  = 2 << 20
	AllDone             string = "__ALL_DONE__"
	HealthCheck         string = "__HEALTHCHECK__"
)

var (
	symLinkMap    map[string]any = make(map[string]any, 64)
	folderInfoMap map[string]any = make(map[string]any, 256)
)
