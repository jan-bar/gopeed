package main

import (
	"encoding/json"
	"flag"
	"fmt"
	stdHttp "net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GopeedLab/gopeed/pkg/base"
	"github.com/GopeedLab/gopeed/pkg/download"
	"github.com/GopeedLab/gopeed/pkg/protocol/bt"
	"github.com/GopeedLab/gopeed/pkg/protocol/ftp"
	"github.com/GopeedLab/gopeed/pkg/protocol/http"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/net/http/httpproxy"
)

//goland:noinspection GoUnhandledErrorResult
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
	skipFile := flag.Bool("S", false, "Skip File Download")
	flag.Parse()

	reqUrl := flag.Arg(0)
	if reqUrl == "" {
		panic("url is empty")
	}

	if *file != "" {
		tp, tf := filepath.Split(*file)
		if tp != "" {
			*path, *file = tp, tf
		}

		if *overwrite {
			fp := filepath.Join(*path, *file)

			if *skipFile {
				if fi, err := os.Stat(fp); err == nil {
					sp, err := stdHttp.Head(reqUrl)
					if err == nil {
						err = sp.Body.Close()
						s, _ := strconv.ParseInt(sp.Header.Get("Content-Length"), 10, 64)
						if fi.Size() == s {
							fmt.Println("Skipped")
							return
						}
					}
				}
			}

			err = os.RemoveAll(fp)
			if err != nil {
				panic(err)
			}
		}
	}

	var pURL *url.URL
	if *proxy == "" {
		getEnvAny := func(names ...string) string {
			for _, n := range names {
				if val := os.Getenv(n); val != "" {
					return val
				}
			}
			return ""
		}
		cfg := &httpproxy.Config{
			HTTPProxy:  getEnvAny("ALL_PROXY", "all_proxy", "HTTP_PROXY", "http_proxy"),
			HTTPSProxy: getEnvAny("ALL_PROXY", "all_proxy", "HTTPS_PROXY", "https_proxy"),
			NoProxy:    getEnvAny("no_proxy", "NO_PROXY"),
			CGI:        os.Getenv("REQUEST_METHOD") != "",
		}

		pURL, err = url.Parse(reqUrl)
		if err != nil {
			panic(err)
		}

		pURL, err = cfg.ProxyFunc()(pURL)
		if err != nil {
			panic(err)
		}
	} else if *proxy != "off" {
		pURL, err = url.Parse(*proxy)
		if err != nil {
			panic(err)
		}
	}

	var reqProxy *base.RequestProxy
	if pURL != nil {
		reqProxy = &base.RequestProxy{
			Mode:   base.RequestProxyModeCustom,
			Scheme: pURL.Scheme,
			Host:   pURL.Host,
		}

		if pwd, ok := pURL.User.Password(); ok {
			reqProxy.Pwd = pwd
			reqProxy.Usr = pURL.User.Username()
		}
	}

	var selectFiles []int
	if *index != "" {
		for v := range strings.SplitSeq(*index, ",") {
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
		if t, v, ok := strings.Cut(*extra, "="); ok {
			switch t {
			case "bt":
				tmp = new(bt.ReqExtra)
			case "ftp":
				tmp = new(ftp.ReqExtra)
			case "http":
				tmp = new(http.ReqExtra)
			}
			if tmp != nil {
				if json.Unmarshal([]byte(v), tmp) == nil {
					reqExtra = tmp
				}
			}
		}
	}

	down := download.NewDownloader(nil)
	err = down.Setup()
	if err != nil {
		panic(err)
	}

	done := make(chan struct{}, 1)
	bar := progressbar.NewOptions64(-2,
		progressbar.OptionSetDescription("downloading"),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetElapsedTime(true),
		progressbar.OptionShowCount(),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionUseANSICodes(true),
		progressbar.OptionSetTheme(progressbar.ThemeASCII),
	)
	down.Listener(func(event *download.Event) {
		if bar.GetMax64() < 0 && event.Task.Meta.Res != nil && event.Task.Meta.Res.Size > 0 {
			bar.ChangeMax64(event.Task.Meta.Res.Size)
		}
		if event.Task.Progress != nil {
			bar.Set64(event.Task.Progress.Downloaded)
		}
		if event.Key == download.EventKeyFinally {
			if event.Err != nil {
				bar.Describe("fail")
				fmt.Printf("\nreason: %s\n", event.Err.Error())
			} else {
				bar.Describe("complete")
				fmt.Printf("\nsaving file: %s\n", filepath.Clean(event.Task.Meta.SingleFilepath()))
			}
			bar.Finish()
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
				AutoTorrent: autoTorrent,
			},
		})
	if err != nil {
		panic(err)
	}
	<-done
}
