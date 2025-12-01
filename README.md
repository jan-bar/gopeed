# gopeed

```shell
# Linux
CGO_ENABLED=0 go install -ldflags "-s -w" -trimpath "github.com/jan-bar/gopeed@$(curl -s https://api.github.com/repos/jan-bar/gopeed/releases/latest | jq -r .tag_name)"
# Windows
set CGO_ENABLED=0& go install -ldflags "-s -w" -trimpath github.com/jan-bar/gopeed@latest
```
