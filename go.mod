module rpcopy

go 1.26.2

replace pb => ./pb

require (
	github.com/klauspost/compress v1.18.6
	github.com/spf13/cobra v1.10.2
	github.com/zeebo/xxh3 v1.1.0
	google.golang.org/grpc v1.81.0
	pb v0.0.0-00010101000000-000000000000
)

require (
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
