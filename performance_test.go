package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/notnil/chess"
)

type budgetEngine struct {
	calls   int
	budgets []int
}

func (e *budgetEngine) Analyse(fen string, ms int) (evaluation, error) {
	e.calls++
	e.budgets = append(e.budgets, ms)
	g, _ := gameAt(fen)
	moves := g.ValidMoves()
	v := evaluation{}
	if len(moves) > 0 {
		v.Move = moves[0].String()
	}
	return v, nil
}
func TestRepeatedPositionsReuseAnalysis(t *testing.T) {
	e := &budgetEngine{}
	p, err := buildProfile([]rawGame{{PGN: demo}, {PGN: demo}}, "Target", e, nil)
	if err != nil || p.Games != 2 {
		t.Fatal(p, err)
	}
	if e.calls > 8 {
		t.Fatalf("searched shared positions again %d", e.calls)
	}
	for _, ms := range e.budgets {
		if ms > 10 {
			t.Fatal("analysis exceeded fast search budget")
		}
	}
}
func TestMoveLimitAndAccurateProgress(t *testing.T) {
	g := chess.NewGame()
	for i := 0; i < 80; i++ {
		moves := g.ValidMoves()
		if len(moves) == 0 {
			break
		}
		g.Move(moves[i%len(moves)])
	}
	raw := "[White \"Target\"]\n[Black \"Other\"]\n" + g.String()
	e := &budgetEngine{}
	last, total := 0, 0
	p, err := buildProfile([]rawGame{{PGN: raw}}, "Target", e, func(done, n int) {
		if done < last || done > n {
			t.Fatal("invalid progress")
		}
		last, total = done, n
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.Analysed > 24 || last != total || total > 24 {
		t.Fatal(p.Analysed, last, total)
	}
}
func TestRecommendationUsesAvailableGames(t *testing.T) {
	for _, n := range []int{1, 2, 3, 8} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			games := make([]rawGame, n)
			for i := range games {
				games[i].PGN = demo
			}
			r := recommendedGames(games)
			if r < 1 || r > n || r > 3 {
				t.Fatal(r)
			}
		})
	}
}
func TestCachedGamesWorkOffline(t *testing.T) {
	a := newApp(t.TempDir())
	path := filepath.Join(a.dir, "games", "target.json")
	atomicJSON(path, sample{[]rawGame{{PGN: demo}}, time.Now().Unix()})
	games, cached, err := a.recent(context.Background(), "Target")
	if err != nil || !cached || len(games) != 1 {
		t.Fatal(games, cached, err)
	}
	code, data := request(t, a, "POST", "/api/preview", map[string]string{"username": "Target"})
	if code != 200 || !strings.Contains(string(data), `"recommended":1`) {
		t.Fatal(code, string(data))
	}
}
func TestPreviewImportsThenReusesCache(t *testing.T) {
	a := newApp(t.TempDir())
	code, data := request(t, a, "POST", "/api/preview", map[string]string{"username": "Target", "filename": "games.pgn", "content": demo})
	if code != 200 {
		t.Fatal(string(data))
	}
	_, cached, err := a.recent(context.Background(), "Target")
	if err != nil || !cached {
		t.Fatal(err)
	}
}
func TestPrivatePathsAndSmallState(t *testing.T) {
	a := newApp(t.TempDir())
	a.settings.Folder = "private-user-folders"
	a.settings.Engine = "private-engine-location"
	a.current = fixture(t)
	code, data := request(t, a, "GET", "/api/state", nil)
	if code != 200 || strings.Contains(string(data), "private-user") || strings.Contains(string(data), "private-engine") || strings.Contains(string(data), `"moves"`) {
		t.Fatal("state exposed private paths or full learned move data")
	}
	code, data = request(t, a, "GET", "/api/settings", nil)
	if code != 200 || !strings.Contains(string(data), "private-user-folders") {
		t.Fatal("owner cannot view settings")
	}
	for _, endpoint := range []string{"state", "settings", "progress", "export", "hardware"} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:8765/api/"+endpoint, nil)
		w := httptest.NewRecorder()
		a.handler().ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("anonymous settings access", endpoint)
		}
	}
}
func TestLightweightProgressEndpoint(t *testing.T) {
	a := newApp(t.TempDir())
	a.current = fixture(t)
	a.job = job{Busy: true, Percent: 53, Message: "Analysing moves"}
	code, data := request(t, a, "GET", "/api/progress", nil)
	if code != 200 || len(data) > 250 || !strings.Contains(string(data), `"percent":53`) {
		t.Fatal(code, string(data))
	}
}
func TestAdviceMatchesHistoricalMistake(t *testing.T) {
	p := fixture(t)
	g := chess.NewGame()
	for _, san := range []string{"e4", "e5", "Qh5", "Nc6", "Bc4", "Nf6"} {
		g.MoveStr(san)
	}
	p.Puzzles = []puzzle{{FEN: g.FEN(), Best: "h5f7", Played: "g8f6", Loss: 1000}}
	e := &budgetEngine{}
	text, err := positionAdvice(p, g.Position(), e)
	if err != nil || !strings.Contains(text, "Qxf7#") || !strings.Contains(text, "mate") || !strings.Contains(text, "saved game") || e.calls != 0 {
		t.Fatal(text, err)
	}
}
func TestAdviceReturnsNormalStockfishMove(t *testing.T) {
	e := &budgetEngine{}
	text, err := positionAdvice(fixture(t), chess.StartingPosition(), e)
	if err != nil || !strings.Contains(text, "Play ") || e.calls != 1 {
		t.Fatal(text, err)
	}
}
func TestShortCoachText(t *testing.T) {
	s := shortText(strings.Repeat("A word. → More: words; ", 20))
	if len(strings.Fields(s)) > 40 || strings.ContainsAny(s, ".·:;→←↓…") {
		t.Fatal(s)
	}
}
func TestGPUOptionAffectsOllamaPayload(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					Options   map[string]int `json:"options"`
					KeepAlive string         `json:"keep_alive"`
				}
				json.NewDecoder(r.Body).Decode(&payload)
				n, present := payload.Options["num_gpu"]
				if enabled && present || !enabled && (!present || n != 0) {
					t.Error("GPU preference was ignored")
				}
				if payload.Options["num_predict"] > 80 || payload.KeepAlive != "30s" {
					t.Error("unbounded model resource settings")
				}
				fmt.Fprint(w, `{"response":"Play e4"}`)
			}))
			defer server.Close()
			_, err := coach(context.Background(), fixture(t), settings{Provider: "ollama", Endpoint: server.URL, Model: "local", UseGPU: enabled})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestGPUActiveRequiresSelectedModelVRAM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"models":[{"name":"local","size_vram":1024}]}`)
	}))
	defer server.Close()
	s := settings{Provider: "ollama", Endpoint: server.URL, Model: "local"}
	g := gpuUsage(context.Background(), s)
	if !g.Active {
		t.Fatal("missed active GPU")
	}
	s.Model = "other"
	if gpuUsage(context.Background(), s).Active {
		t.Fatal("claimed GPU use for unloaded model")
	}
}
func TestNoPersonalPathsInSharedUI(t *testing.T) {
	for _, name := range []string{"ui/index.html", "ui/app.js", "README.md", "CONTRIBUTING.md"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(data)), `c:\users\`) || strings.Contains(strings.ToLower(string(data)), "/users/") {
			t.Fatal("shared content has a user directory", name)
		}
	}
}
func TestLearnedMoveNeedsNoEngineProcess(t *testing.T) {
	a := newApp(t.TempDir())
	a.current = fixture(t)
	a.settings.Engine = "not-installed"
	code, data := request(t, a, "POST", "/api/play", map[string]any{"reset": true, "color": "black"})
	if code != 200 || a.playEngine != nil || !strings.Contains(string(data), "learned") {
		t.Fatal(code, string(data))
	}
}
