mkdir -p dist/cert/server
mkdir -p dist/cert/client
cp -r cert/ca.crt dist/cert/ca.crt
cp -r cert/server/server.* dist/cert/server/
cp -r cert/client/client.* dist/cert/client/

zip -r dist/cert_files_zcp_corpnet.zip dist/cert -x ".DS_Store"

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/windows_amd64/zcp.exe -ldflags "-w -s" main.go
zip dist/windows_amd64/zcp_windows_amd64.zip dist/windows_amd64/zcp.exe

CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/macos_arm/zcp -ldflags "-w -s" main.go
zip dist/macos_arm/zcp_macos_arm.zip dist/macos_arm/zcp

CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/macos_intel/zcp -ldflags "-w -s" main.go
zip dist/macos_intel/zcp_macos_intel.zip dist/macos_intel/zcp


CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/linux_amd64/zcp -ldflags "-w -s" main.go
zip dist/linux_amd64/zcp_linux_amd64.zip dist/linux_amd64/zcp

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o dist/linux_arm64/zcp -ldflags "-w -s" main.go
zip dist/linux_arm64/zcp_linux_arm64.zip dist/linux_arm64/zcp


