package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/notnil/chess"
)

type sample struct {
	Games   []rawGame `json:"games"`
	Fetched int64     `json:"fetched"`
}

func (a *app) recent(ctx context.Context, username string) ([]rawGame, bool, error) {
	if !usernameRE.MatchString(username) {
		return nil, false, fmt.Errorf("enter a valid username")
	}
	path := filepath.Join(a.dir, "games", strings.ToLower(username)+".json")
	var saved sample
	if data, e := os.ReadFile(path); e == nil && json.Unmarshal(data, &saved) == nil && len(saved.Games) > 0 && time.Since(time.Unix(saved.Fetched, 0)) < 24*time.Hour {
		return saved.Games, true, nil
	}
	games, err := fetchGames(ctx, &http.Client{Timeout: 12 * time.Second}, "https://api.chess.com/pub/player", username, 8)
	if err != nil {
		return nil, false, err
	}
	if len(games) == 0 {
		return nil, false, fmt.Errorf("no recent standard games found")
	}
	if err = atomicJSON(path, sample{games, time.Now().Unix()}); err != nil {
		return nil, false, fmt.Errorf("cannot save recent games")
	}
	return games, false, nil
}
func recommendedGames(games []rawGame) int {
	budget := 0
	count := 0
	for _, raw := range games {
		option, err := chess.PGN(strings.NewReader(raw.PGN))
		if err != nil {
			continue
		}
		work := min(24, (len(chess.NewGame(option).Moves())+1)/2) * 20
		if count > 0 && budget+work > 1200 {
			break
		}
		budget += work
		count++
		if count == 3 {
			break
		}
	}
	return max(1, count)
}

// Called while the app mutex is held so only one search uses this engine at a time.
func (a *app) Analyse(fen string, ms int) (evaluation, error) {
	if a.playEngine == nil {
		e, err := newEngine(a.settings.Engine)
		if err != nil {
			return evaluation{}, err
		}
		a.playEngine = e
	}
	value, err := a.playEngine.Analyse(fen, ms)
	if err != nil {
		a.playEngine.Close()
		a.playEngine = nil
	}
	return value, err
}
func (a *app) closeEngine() {
	if a.playEngine != nil {
		a.playEngine.Close()
		a.playEngine = nil
	}
}
func profileSummary(p *profile) any {
	if p == nil {
		return nil
	}
	return map[string]any{"username": p.Username, "games": p.Games, "analysed_moves": p.Analysed, "puzzle_count": len(p.Puzzles), "openings": p.Openings, "record": p.Record, "notes": report(p)}
}
