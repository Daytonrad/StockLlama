package main

import (
	"fmt"
	"strings"

	"github.com/notnil/chess"
)

func shortText(s string) string {
	s = strings.NewReplacer(".", " ", "·", " ", "→", " ", "←", " ", "↓", " ", "…", " ", ";", " ", ":", " ").Replace(s)
	words := strings.Fields(s)
	return strings.Join(words[:min(40, len(words))], " ")
}
func movePurpose(pos *chess.Position, m *chess.Move) string {
	san := (chess.AlgebraicNotation{}).Encode(pos, m)
	if strings.HasSuffix(san, "#") {
		return "deliver mate"
	}
	piece := pos.Board().Piece(m.S2()).Type()
	if piece != chess.NoPieceType {
		return "capture the " + map[chess.PieceType]string{chess.Queen: "queen", chess.Rook: "rook", chess.Bishop: "bishop", chess.Knight: "knight", chess.Pawn: "pawn", chess.King: "king"}[piece]
	}
	if strings.HasSuffix(san, "+") {
		return "give check"
	}
	return "improve the position according to Stockfish"
}
func positionAdvice(p *profile, pos *chess.Position, e analyser) (string, error) {
	if len(pos.ValidMoves()) == 0 {
		return "Game finished Start a new game to continue", nil
	}
	for _, q := range p.Puzzles {
		g, err := gameAt(q.FEN)
		if err != nil || key(g.Position()) != key(pos) {
			continue
		}
		m, err := legalMove(pos, q.Best)
		if err != nil {
			continue
		}
		return fmt.Sprintf("Play %s to %s This reply was available after %s played %s in a saved game", (chess.AlgebraicNotation{}).Encode(pos, m), movePurpose(pos, m), p.Username, q.Played), nil
	}
	v, err := e.Analyse(pos.String(), 30)
	if err != nil {
		return "Stockfish unavailable " + report(p), nil
	}
	m, err := legalMove(pos, v.Move)
	if err != nil {
		return "", err
	}
	note := fmt.Sprintf("Play %s to %s", (chess.AlgebraicNotation{}).Encode(pos, m), movePurpose(pos, m))
	if len(p.Puzzles) > 0 {
		return note + fmt.Sprintf(" Study %d saved mistakes from %s before playing them", len(p.Puzzles), p.Username), nil
	}
	return note + " No matching mistake was found in the saved sample", nil
}
