package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	stdHttp "net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	"github.com/GopeedLab/gopeed/pkg/download"
	"github.com/GopeedLab/gopeed/pkg/protocol/bt"
	"github.com/GopeedLab/gopeed/pkg/protocol/ftp"
	"github.com/GopeedLab/gopeed/pkg/protocol/http"
	"github.com/cheggaaa/pb/v3"
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

	var info *metadata
	if *file != "" {
		tp, tf := filepath.Split(*file)
		if tp != "" {
			*path, *file = tp, tf
		}

		fp := filepath.Join(*path, *file)
		if *skipFile && headFile(reqUrl, fp, pURL, *skipVerify, &info) {
			fmt.Println("Skipped")
			os.Exit(3)
		}
		if *overwrite {
			err = os.RemoveAll(fp)
			err = os.Remove(fp + ".json")
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
	bar := pb.New64(0).
		SetWidth(100).
		SetTemplateString(`{{string . "prefix"}} {{percent . "%.0f%%"}} {{bar . "[" "=" ">" "." "]"}} ({{counters . "%s/%s"}}, {{speed .}}) [{{etime .}}{{if and (not .IsFinished) (gt .Total 0)}}:{{rtime .}}{{end}}]`).
		Set(pb.Bytes, true).
		Set(pb.SIBytesPrefix, true).
		Set(pb.Color, false).
		Set(pb.Static, true).
		Set(pb.TimeRound, time.Second).
		Set("prefix", "downloading").
		SetWriter(os.Stdout).
		Start()
	down.Listener(func(event *download.Event) {
		if bar.Total() == 0 && event.Task.Meta.Res != nil && event.Task.Meta.Res.Size > 0 {
			bar.SetTotal(event.Task.Meta.Res.Size)
		}
		if event.Task.Progress != nil {
			bar.SetCurrent(event.Task.Progress.Downloaded)
		}
		if event.Key == download.EventKeyFinally {
			if event.Err != nil {
				bar.Set("prefix", "fail").Finish().Write()
				fmt.Printf("\nreason: %s\n", event.Err.Error())
			} else {
				fp := filepath.Clean(event.Task.Meta.SingleFilepath())
				if info != nil {
					if err := saveFileMetadata(fp, info); err != nil {
						fmt.Printf("\nfailed to save file metadata: %s\n", err)
					}
				}
				bar.Set("prefix", "complete").Finish().Write()
				fmt.Printf("\nsaving file: %s\n", fp)
			}
			done <- struct{}{}
		} else {
			bar.Write()
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

type metadata struct {
	ETag          string `json:"etag"`
	LastModified  string `json:"last-modified"`
	ContentLength *int64 `json:"content-length"`
	LocalSize     *int64 `json:"local-size"`
}

func headFile(u, fp string, proxy *url.URL, skipVerify bool, v **metadata) bool {
	remote, err := getRemoteMetadata(u, proxy, skipVerify)
	if err != nil {
		return false
	}
	*v = &remote

	fi, err := os.Stat(fp)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}

	data, err := os.ReadFile(fp + ".json")
	if err != nil {
		return false
	}

	var info metadata
	if err = json.Unmarshal(data, &info); err != nil {
		return false
	}

	if info.LocalSize != nil && fi.Size() != *info.LocalSize {
		return false
	}

	if remote.ContentLength != nil {
		if info.ContentLength != nil && *remote.ContentLength != *info.ContentLength {
			return false
		}
	}

	if remote.ETag != "" && info.ETag != "" && remote.ETag != info.ETag {
		return false
	}

	if remote.LastModified != "" && info.LastModified != "" && remote.LastModified != info.LastModified {
		return false
	}

	return (remote.ETag != "" && info.ETag != "") || (remote.LastModified != "" && info.LastModified != "")
}

func saveFileMetadata(fp string, info *metadata) error {
	fi, err := os.Stat(fp)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("downloaded path is not a regular file: %s", fp)
	}

	localSize := fi.Size()
	info.LocalSize = &localSize
	data, err := json.MarshalIndent(info, "", "\t")
	if err != nil {
		return err
	}

	f := fp + ".json"
	tmp := f + ".tmp"

	if err = os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmp, f)
}

func getRemoteMetadata(u string, proxy *url.URL, skipVerify bool) (metadata, error) {
	req, err := stdHttp.NewRequest(stdHttp.MethodHead, u, nil)
	if err != nil {
		return metadata{}, err
	}

	c := &stdHttp.Client{Timeout: 10 * time.Second}
	if proxy != nil || skipVerify {
		c.Transport = &stdHttp.Transport{
			Proxy: stdHttp.ProxyURL(proxy),
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipVerify,
			},
		}
	}

	resp, err := c.Do(req)
	if err != nil {
		return metadata{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != stdHttp.StatusOK {
		return metadata{}, fmt.Errorf("unexpected HEAD status: %s", resp.Status)
	}

	info := metadata{
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}
	if resp.ContentLength >= 0 {
		info.ContentLength = &resp.ContentLength
	}
	return info, nil
}
