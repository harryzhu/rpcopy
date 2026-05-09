mkdir -p dist/cert/server
mkdir -p dist/cert/client
cp -r cert/ca.crt dist/cert/ca.crt
cp -r cert/server/server.* dist/cert/server/
cp -r cert/client/client.* dist/cert/client/

zip -r dist/cert_files_rpcopy_com.zip dist/cert -x ".DS_Store"

CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o dist/macos_arm/rpcopy -ldflags "-w -s" main.go
zip dist/macos_arm/rpcopy_macos_arm.zip dist/macos_arm/rpcopy

CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/macos_intel/rpcopy -ldflags "-w -s" main.go
zip dist/macos_intel/rpcopy_macos_intel.zip dist/macos_intel/rpcopy


CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/linux_amd64/rpcopy -ldflags "-w -s" main.go
zip dist/linux_amd64/rpcopy_linux_amd64.zip dist/linux_amd64/rpcopy


CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o dist/windows_amd64/rpcopy.exe -ldflags "-w -s" main.go
zip dist/windows_amd64/rpcopy_windows_amd64.zip dist/windows_amd64/rpcopy.exe
