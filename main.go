package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
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
	connections := flag.Int("C", 16, "Concurrent connections")
	autoTorrent := flag.Bool("A", false, "Auto create a new task for the torrent file")
	overwrite := flag.Bool("O", false, "Overwrite existing file")
	flag.Parse()

	url := flag.Arg(0)
	if url == "" {
		panic("url is empty")
	}

	var selectFiles []int
	if *index != "" {
		for _, v := range strings.Split(*index, ",") {
			if i, err := strconv.Atoi(v); err == nil {
				selectFiles = append(selectFiles, i)
			}
		}
	}

	var extraReq any
	if *extra != "" {
		if strings.Contains(url, `"trackers"`) {
			req := new(bt.ReqExtra)
			if json.Unmarshal([]byte(*extra), req) == nil {
				extraReq = req
			}
		} else {
			req := new(http.ReqExtra)
			if json.Unmarshal([]byte(*extra), req) == nil {
				extraReq = req
			}
		}
	}

	if *overwrite {
		err = os.RemoveAll(filepath.Join(*path, *file))
		if err != nil {
			panic(err)
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

	_, err = download.Boot().
		URL(url).
		Listener(func(event *download.Event) {
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
		}).
		Extra(extraReq).
		Create(&base.Options{
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
