package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeGitHub serves canned bodies by URL without touching the network.
type fakeGitHub map[string][]byte

func (f fakeGitHub) RoundTrip(r *http.Request) (*http.Response, error) {
	body, ok := f[r.URL.String()]
	status := http.StatusOK
	if !ok {
		status = http.StatusNotFound
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Header: http.Header{}, Request: r}, nil
}

func TestInstallEngineVerifiesDigest(t *testing.T) {
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	w, _ := z.Create("stockfish/stockfish")
	w.Write([]byte("fake engine"))
	z.Close()
	sum := sha256.Sum256(archive.Bytes())
	good := "sha256:" + hex.EncodeToString(sum[:])

	platform, arch := runtime.GOOS, "x86-64"
	if platform == "darwin" {
		platform = "macos"
	}
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}
	name := "stockfish-" + platform + "-" + arch + "-universal.zip"
	download := "https://github.com/official-stockfish/Stockfish/releases/download/sf_test/" + name

	for _, tc := range []struct {
		name, digest string
		ok           bool
	}{{"matching", good, true}, {"mismatched", "sha256:" + strings.Repeat("0", 64), false}, {"missing", "", false}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cwd, _ := os.Getwd()
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chdir(cwd) })

			release, _ := json.Marshal(map[string]any{"assets": []map[string]string{{"name": name, "browser_download_url": download, "digest": tc.digest}}})
			original := apiClient
			apiClient = &http.Client{Transport: fakeGitHub{
				"https://api.github.com/repos/official-stockfish/Stockfish/releases/latest": release,
				download: archive.Bytes(),
			}}
			t.Cleanup(func() { apiClient = original })

			path, err := installEngine(context.Background())
			if tc.ok {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
					t.Fatal("engine not installed:", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "checksum") {
				t.Fatalf("want checksum error, got %v", err)
			}
			if entries, _ := os.ReadDir(filepath.Join(dir, "engines")); len(entries) != 0 {
				t.Fatalf("files left after a failed check: %v", entries)
			}
		})
	}
}
