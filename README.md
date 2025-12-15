# gopeed

```shell
# Linux
CGO_ENABLED=0 go install -ldflags "-s -w" -trimpath "github.com/jan-bar/gopeed@$(curl -s https://api.github.com/repos/jan-bar/gopeed/releases/latest | jq -r .tag_name)"
wget -O gopeed "https://github.com/jan-bar/gopeed/releases/latest/download/gopeed_linux_amd64"
# Windows
set CGO_ENABLED=0& go install -ldflags "-s -w" -trimpath github.com/jan-bar/gopeed@latest
wget -O gopeed.exe "https://github.com/jan-bar/gopeed/releases/latest/download/gopeed_windows_amd64.exe"
```
