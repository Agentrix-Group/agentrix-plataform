// Package tournament defines the versioned, complete five-player round format.
package tournament

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

const Version = "all-cohorts-5-seats-v1"

type Plan struct {
	Roster []int
	Seed   uint32
	Total  int
}
type Match struct {
	Agents [5]int
	Seed   uint32
}

func Choose(n, k int) int {
	if n < k || k < 0 {
		return 0
	}
	value := 1
	for i := 1; i <= k; i++ {
		value = value * (n - k + i) / i
	}
	return value
}

func New(roster []int, seed uint32) (Plan, error) {
	seen := map[int]bool{}
	for _, id := range roster {
		if id <= 0 || seen[id] {
			return Plan{}, errors.New("round roster must contain distinct valid agent IDs")
		}
		seen[id] = true
	}
	if len(roster) > 20 {
		return Plan{}, errors.New("complete round format supports up to 20 teams")
	}
	return Plan{Roster: append([]int(nil), roster...), Seed: seed, Total: 5 * Choose(len(roster), 5)}, nil
}

// At un-ranks a combination without materializing the full plan. Even a round
// of 20 teams uses a small roster and bounded queue rather than 77520 records.
func (p Plan) At(index int) (Match, error) {
	if index < 0 || index >= p.Total {
		return Match{}, errors.New("round index out of range")
	}
	rank, rotation := index/5, index%5
	var combo [5]int
	start := 0
	for pos := 0; pos < 5; pos++ {
		for candidate := start; candidate < len(p.Roster); candidate++ {
			ways := Choose(len(p.Roster)-candidate-1, 4-pos)
			if rank < ways {
				combo[pos] = candidate
				start = candidate + 1
				break
			}
			rank -= ways
		}
	}
	var match Match
	for seat := 0; seat < 5; seat++ {
		match.Agents[seat] = p.Roster[combo[(seat+rotation)%5]]
	}
	var source [8]byte
	binary.BigEndian.PutUint32(source[:4], p.Seed)
	binary.BigEndian.PutUint32(source[4:], uint32(index))
	digest := sha256.Sum256(source[:])
	match.Seed = binary.BigEndian.Uint32(digest[:4])
	return match, nil
}
