package main

import (
	"context"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type gpuStatus struct {
	Detected bool   `json:"detected"`
	Active   bool   `json:"active"`
	Name     string `json:"name"`
}

var gpuOnce sync.Once
var localGPU gpuStatus

func detectGPU() gpuStatus {
	gpuOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		defer cancel()
		out, err := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name", "--format=csv,noheader").Output()
		if err == nil && strings.TrimSpace(string(out)) != "" {
			localGPU = gpuStatus{Detected: true, Name: strings.Split(strings.TrimSpace(string(out)), "\n")[0]}
		}
		if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
			localGPU = gpuStatus{Detected: true, Name: "Apple GPU"}
		}
	})
	return localGPU
}
func gpuUsage(ctx context.Context, s settings) gpuStatus {
	result := detectGPU()
	if s.Provider != "ollama" {
		return result
	}
	var response struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
			VRAM  int64  `json:"size_vram"`
		} `json:"models"`
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if modelRequest(ctx, s, http.MethodGet, "/api/ps", nil, &response) == nil {
		for _, m := range response.Models {
			if m.Name == s.Model || m.Model == s.Model {
				result.Active = m.VRAM > 0
				if result.Active {
					result.Detected = true
					if result.Name == "" {
						result.Name = "Ollama GPU"
					}
				}
			}
		}
	}
	return result
}
