package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GopeedLab/gopeed/pkg/base"
	"github.com/GopeedLab/gopeed/pkg/download"
	"github.com/GopeedLab/gopeed/pkg/protocol/bt"
	"github.com/GopeedLab/gopeed/pkg/protocol/http"
	"github.com/GopeedLab/gopeed/pkg/util"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	file := flag.String("F", "", "Download file name")
	path := flag.String("D", dir, "Download file path")
	index := flag.String("I", "", "Select file indexes to download")
	extra := flag.String("E", "", "Extra options for the protocol")
	proxy := flag.String("P", "", "Proxy URL")
	connections := flag.Int("C", 16, "Concurrent connections")
	autoTorrent := flag.Bool("A", false, "Auto create a new task for the torrent file")
	overwrite := flag.Bool("O", false, "Overwrite existing file")
	skipVerify := flag.Bool("K", false, "Skip verify cert")
	flag.Parse()

	reqUrl := flag.Arg(0)
	if reqUrl == "" {
		panic("url is empty")
	}

	if *overwrite {
		err = os.RemoveAll(filepath.Join(*path, *file))
		if err != nil {
			panic(err)
		}
	}

	var reqProxy *base.RequestProxy
	if *proxy != "" {
		u, err := url.Parse(*proxy)
		if err != nil {
			panic(err)
		}

		reqProxy = &base.RequestProxy{
			Mode:   base.RequestProxyModeCustom,
			Scheme: u.Scheme,
			Host:   u.Host,
		}

		if pwd, ok := u.User.Password(); ok {
			reqProxy.Pwd = pwd
			reqProxy.Usr = u.User.Username()
		}
	}

	var selectFiles []int
	if *index != "" {
		for _, v := range strings.Split(*index, ",") {
			if i, err := strconv.Atoi(v); err == nil {
				selectFiles = append(selectFiles, i)
			}
		}
	}

	var reqExtra any
	if *extra != "" {
		if d, err := os.ReadFile(*extra); err == nil {
			*extra = string(d)
		}

		var tmp any
		if strings.Contains(*extra, `"trackers"`) {
			tmp = new(bt.ReqExtra)
		} else {
			tmp = new(http.ReqExtra)
		}

		if json.Unmarshal([]byte(*extra), tmp) == nil {
			reqExtra = tmp
		}
	}

	const progressWidth = 20

	var (
		sb = bytes.NewBuffer(make([]byte, 0, 128))

		lastLineLen   = 0
		printProgress = func(task *download.Task, title string) {
			var (
				total int64
				rate  float64
			)

			if task.Meta.Res != nil && task.Meta.Res.Size > 0 {
				total = task.Meta.Res.Size
				rate = float64(task.Progress.Downloaded) / float64(task.Meta.Res.Size)
			}

			fmt.Fprintf(sb, "\r%s [", title)

			i := 0
			for ; i < int(progressWidth*rate); i++ {
				sb.WriteString("■")
			}
			for ; i < progressWidth; i++ {
				sb.WriteString("□")
			}

			fmt.Fprintf(sb, "] %.1f%%    %s/s    %s", rate*100,
				util.ByteFmt(task.Progress.Speed),
				util.ByteFmt(total))

			if lastLineLen != 0 {
				for i = lastLineLen - sb.Len(); i > 0; i-- {
					sb.WriteByte(' ')
				}
			}

			lastLineLen = sb.Len()
			os.Stdout.Write(sb.Bytes())
			sb.Reset()
		}

		done = make(chan struct{}, 1)
	)

	down := download.NewDownloader(nil)

	err = down.Setup()
	if err != nil {
		panic(err)
	}

	down.Listener(func(event *download.Event) {
		switch event.Key {
		case download.EventKeyProgress:
			printProgress(event.Task, "downloading")
		case download.EventKeyFinally:
			if event.Err != nil {
				printProgress(event.Task, "fail")
				fmt.Printf("\nreason: %s\n", event.Err.Error())
			} else {
				printProgress(event.Task, "complete")
				fmt.Printf("\nsaving file: %s\n",
					filepath.FromSlash(event.Task.Meta.SingleFilepath()))
			}
			done <- struct{}{}
		}
	})

	_, err = down.CreateDirect(
		&base.Request{
			URL:            reqUrl,
			Extra:          reqExtra,
			Proxy:          reqProxy,
			SkipVerifyCert: *skipVerify,
		},
		&base.Options{
			Name:        *file,
			Path:        *path,
			SelectFiles: selectFiles,
			Extra: &http.OptsExtra{
				Connections: *connections,
				AutoTorrent: *autoTorrent,
			},
		})
	if err != nil {
		panic(err)
	}
	<-done
}
