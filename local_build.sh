CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/macos_arm/zcp -ldflags "-w -s" main.go
rm -f /Volumes/harry/vendor/bin/zcp
cp dist/macos_arm/zcp /Volumes/harry/vendor/bin/

