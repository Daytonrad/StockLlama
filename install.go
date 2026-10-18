package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type progressReader struct {
	reader      io.Reader
	total, read int64
	notify      func(int)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.reader.Read(b)
	p.read += int64(n)
	if p.total > 0 && p.notify != nil {
		p.notify(int(p.read * 95 / p.total))
	}
	return n, err
}
func installEngine(ctx context.Context, notify ...func(int)) (string, error) {
	var release struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := getJSON(ctx, apiClient, "https://api.github.com/repos/official-stockfish/Stockfish/releases/latest", &release); err != nil {
		return "", fmt.Errorf("cannot fetch official Stockfish release: %w", err)
	}
	arch := "x86-64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	url := ""
	name := ""
	for _, a := range release.Assets {
		if strings.Contains(a.Name, platform+"-"+arch) && strings.Contains(a.Name, "universal") {
			url = a.URL
			name = a.Name
			break
		}
	}
	if url == "" {
		return "", fmt.Errorf("no universal Stockfish build found; download from stockfishchess.org and set its path")
	}
	if !strings.HasPrefix(url, "https://github.com/official-stockfish/Stockfish/releases/download/") {
		return "", fmt.Errorf("unexpected Stockfish download URL")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	res, err := apiClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Stockfish download failed")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("Stockfish download returned %d", res.StatusCode)
	}
	os.MkdirAll("engines", 0700)
	file, err := os.CreateTemp("engines", "download-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	var reader io.Reader = res.Body
	if len(notify) > 0 {
		reader = &progressReader{reader: res.Body, total: res.ContentLength, notify: notify[0]}
	}
	n, err := io.Copy(file, io.LimitReader(reader, 200<<20))
	file.Close()
	if err != nil || n >= 200<<20 {
		return "", fmt.Errorf("engine download incomplete or too large")
	}
	destination := filepath.Join("engines", "stockfish-local")
	if runtime.GOOS == "windows" {
		destination += ".exe"
	}
	// Only extract the engine and its license; do not trust paths from archives.
	write := func(name string, r io.Reader) error {
		base := filepath.Base(name)
		target := ""
		if strings.HasPrefix(base, "stockfish") && (strings.HasSuffix(base, ".exe") || !strings.Contains(base, ".")) {
			target = destination
		}
		if strings.EqualFold(base, "Copying.txt") {
			target = filepath.Join("engines", "Stockfish-LICENSE.txt")
		}
		if target == "" {
			return nil
		}
		f, e := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0700)
		if e != nil {
			return e
		}
		defer f.Close()
		_, e = io.Copy(f, io.LimitReader(r, 150<<20))
		return e
	}
	if strings.HasSuffix(name, ".zip") {
		z, e := zip.OpenReader(file.Name())
		if e != nil {
			return "", e
		}
		defer z.Close()
		for _, entry := range z.File {
			if entry.FileInfo().IsDir() {
				continue
			}
			r, e := entry.Open()
			if e != nil {
				return "", e
			}
			e = write(entry.Name, r)
			r.Close()
			if e != nil {
				return "", e
			}
		}
	} else {
		f, e := os.Open(file.Name())
		if e != nil {
			return "", e
		}
		defer f.Close()
		gz, e := gzip.NewReader(f)
		if e != nil {
			return "", e
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			h, e := tr.Next()
			if e == io.EOF {
				break
			}
			if e != nil {
				return "", e
			}
			if h.Typeflag == tar.TypeReg {
				if e = write(h.Name, tr); e != nil {
					return "", e
				}
			}
		}
	}
	if _, err = os.Stat(destination); err != nil {
		return "", fmt.Errorf("engine executable not found in download")
	}
	return destination, nil
}
