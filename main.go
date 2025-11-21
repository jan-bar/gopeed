package main

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	"github.com/GopeedLab/gopeed/pkg/download"
	"github.com/GopeedLab/gopeed/pkg/protocol/http"
	"github.com/schollz/progressbar/v3"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	file := flag.String("F", "", "Download file name")
	path := flag.String("D", dir, "Download file path")
	index := flag.String("I", "", "Select file indexes to download")
	connections := flag.Int("C", 16, "Concurrent connections.")
	autoTorrent := flag.Bool("A", false, "Auto create a new task for the torrent file")
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

	var (
		sMax sync.Once
		done = make(chan struct{}, 1)
		pBar = progressbar.NewOptions64(
			-1,
			progressbar.OptionSetDescription("downloading"),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionShowBytes(true),
			progressbar.OptionShowTotalBytes(true),
			progressbar.OptionSetWidth(10),
			progressbar.OptionThrottle(65*time.Millisecond),
			progressbar.OptionShowCount(),
			progressbar.OptionSpinnerType(34),
			progressbar.OptionSetWidth(30),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionOnCompletion(func() {
				os.Stderr.WriteString("\n")
				done <- struct{}{}
			}),
		)
	)

	_, err = download.Boot().
		URL(url).
		Listener(func(event *download.Event) {
			if event.Task == nil || event.Task.Meta.Res == nil {
				return
			}

			sMax.Do(func() { pBar.ChangeMax64(event.Task.Meta.Res.Size) })

			switch event.Key {
			case download.EventKeyProgress:
				pBar.Set64(event.Task.Progress.Downloaded)
			case download.EventKeyFinally:
				if event.Err != nil {
					pBar.Describe("failed: " + event.Err.Error())
				} else {
					pBar.Describe("completed")
				}
				pBar.Finish()
			}
		}).
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
