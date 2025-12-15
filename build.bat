del gopeed gopeed_linux_amd64.7z gopeed.exe gopeed_windows_amd64.7z

set CGO_ENABLED=0
set GOARCH=amd64
set GOOS=linux
set GOAMD64=v2
go build -ldflags "-s -w -buildid=" -buildvcs=false -trimpath -o gopeed

set GOOS=windows
set GOAMD64=v3
go build -ldflags "-s -w -buildid=" -buildvcs=false -trimpath -o gopeed.exe

7za a -t7z -mx=9 gopeed_linux_amd64.7z gopeed
7za a -t7z -mx=9 gopeed_windows_amd64.7z gopeed.exe
