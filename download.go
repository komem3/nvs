package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"golang.org/x/net/html"
)

var DownloadCmd = &cobra.Command{
	Use:   "download [version]",
	Short: "Download specify version of Nodejs",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		for _, arg := range args {
			v, err := parseVersionString(arg)
			if err != nil {
				fatal(ctx, err)
			}
			if err := Download(ctx, v); err != nil {
				fatal(ctx, err)
			}
		}
	},
}

var maxWorkers = runtime.NumCPU() * 4

type downloadPath struct {
	path     string
	name     string
	priority int
}

const nodejsURL = "https://nodejs.org/dist/"

func findTarget(ctx context.Context, v *version) (string, error) {
	r, err := http.NewRequest(http.MethodGet, nodejsURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return "", fmt.Errorf("status is %d. response %s", resp.StatusCode, body)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return "", err
	}
	var f func(*html.Node) error
	var downloadpaths []downloadPath
	f = func(n *html.Node) error {
		if n.Type == html.ElementNode && n.Data == "a" {
			data := n.FirstChild.Data
			if !versionRegex.MatchString(data) {
				return nil
			}
			splitVersion := strings.Split(strings.Trim(data, "v/"), ".")
			match, err := compareVersionString(splitVersion, v)
			if err != nil {
				debugf(ctx, "%s is skipped: %v", data, err)
			}
			if match {
				downloadpaths = append(downloadpaths, downloadPath{
					path:     strings.TrimRight(data, "/"),
					priority: calcPriority(splitVersion),
				})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := f(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := f(doc); err != nil {
		return "", err
	}

	if len(downloadpaths) == 0 {
		return "", fmt.Errorf("no much version")
	}

	slices.SortFunc(downloadpaths, func(l, r downloadPath) int {
		return r.priority - l.priority
	})

	return downloadpaths[0].path, nil
}

func calcPriority(splitVersion []string) int {
	return mustParse(splitVersion[0])*10000 + mustParse(splitVersion[1])*100 + mustParse(splitVersion[2])
}

func Download(ctx context.Context, v *version) error {
	base, err := checkInit()
	if err != nil {
		return err
	}
	path, err := findTarget(ctx, v)
	if err != nil {
		return err
	}

	downloadFile := fmt.Sprintf("node-%s-%s-%s", path, runtime.GOOS, strings.ReplaceAll(runtime.GOARCH, "amd", "x"))
	url, err := url.JoinPath(nodejsURL, path, downloadFile+".tar.gz")
	if err != nil {
		return err
	}
	infof(ctx, "download %s", url)
	tmpFile, err := download(ctx, url)
	if err != nil {
		return err
	}

	infof(ctx, "extract %s", tmpFile.Name())
	dir, err := extract(tmpFile)
	if err != nil {
		return err
	}

	fromDir := filepath.Join(dir, downloadFile)
	infof(ctx, "copy from %s", fromDir)
	targetPath := filepath.Join(base, "versions", path)
	if err := os.RemoveAll(targetPath); err != nil {
		return fmt.Errorf("remove %s: %w", targetPath, err)
	}
	if err := os.Rename(fromDir, targetPath); err != nil {
		return err
	}

	return nil
}

func extract(file *os.File) (string, error) {
	gr, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("new gzip reader: %w", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, gr); err != nil {
		return "", fmt.Errorf("copy to buffer: %w", err)
	}

	dir := file.Name()[:strings.LastIndex(file.Name(), ".tar.gz")]
	if err := os.Mkdir(dir, 0o755); err != nil {
		return "", err
	}

	tr := tar.NewReader(&buf)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if hdr.Typeflag == tar.TypeDir {
			dir := filepath.Join(dir, hdr.Name)
			if _, err = os.Stat(dir); os.IsNotExist(err) {
				if err := os.Mkdir(dir, hdr.FileInfo().Mode()); err != nil {
					return "", err
				}
			}
			continue
		}
		if hdr.Typeflag == tar.TypeSymlink {
			if err := os.Symlink(hdr.Linkname, filepath.Join(dir, hdr.Name)); err != nil {
				return "", err
			}
			continue
		}

		file, err := os.OpenFile(filepath.Join(dir, hdr.Name), os.O_RDWR|os.O_CREATE|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(file, tr); err != nil {
			file.Close()
			return "", fmt.Errorf("copy to %s: %w", file.Name(), err)
		}
		file.Close()
	}

	return dir, err
}

func download(ctx context.Context, url string) (*os.File, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	size := resp.ContentLength
	chunk := int(size / int64(maxWorkers))
	if size%int64(maxWorkers) != 0 {
		chunk++
	}
	var (
		wg   sync.WaitGroup
		buf  = make([]bytes.Buffer, maxWorkers)
		errs error
	)
	for i := 0; i < maxWorkers; i++ {
		i := i
		wg.Add(1)

		go func() {
			defer wg.Done()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				errs = errors.Join(errs, err)
				return
			}

			if i+1 == maxWorkers {
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-", i*chunk))
			} else {
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", i*chunk, (i+1)*chunk-1))
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				errs = errors.Join(errs, err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusPartialContent {
				errs = errors.Join(errs, fmt.Errorf("response status is %d", resp.StatusCode))
				return
			}
			if _, err := io.Copy(&buf[i], resp.Body); err != nil {
				errs = errors.Join(errs, err)
				return
			}
		}()
	}

	wg.Wait()

	if errs != nil {
		return nil, errs
	}

	tmpFile, err := os.CreateTemp(os.TempDir(), "*.tar.gz")
	if err != nil {
		return nil, err
	}
	for _, b := range buf {
		if _, err := tmpFile.Write(b.Bytes()); err != nil {
			return nil, err
		}
	}
	if err := tmpFile.Close(); err != nil {
		return nil, err
	}
	tmpFile, err = os.Open(tmpFile.Name())
	if err != nil {
		return nil, err
	}
	return tmpFile, nil
}
