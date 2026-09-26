package ladder

import (
	"math"
)

type ParticipantRatingInput struct {
	AgentVersionID int
	OldRating      float64
	RankPlace      int
	Disqualified   bool
	MatchesPlayed  int
}

type ParticipantRatingResult struct {
	AgentVersionID int
	OldRating      float64
	NewRating      float64
	RatingDelta    float64
}

// CalculateMultiplayerElo calculates new ratings for an N-player match (e.g. 5 players).
// Pairwise decomposition guarantees mathematical conservation of total points
// and rewards beating higher-rated bots.
func CalculateMultiplayerElo(participants []ParticipantRatingInput) []ParticipantRatingResult {
	n := len(participants)
	if n < 2 {
		results := make([]ParticipantRatingResult, n)
		for i, p := range participants {
			results[i] = ParticipantRatingResult{
				AgentVersionID: p.AgentVersionID,
				OldRating:      p.OldRating,
				NewRating:      p.OldRating,
				RatingDelta:    0,
			}
		}
		return results
	}

	results := make([]ParticipantRatingResult, n)

	for i := 0; i < n; i++ {
		pA := participants[i]

		// Dynamic K-factor: higher for provisional bots, stabilizes with matches
		k := 24.0
		if pA.MatchesPlayed < 10 {
			k = 48.0
		} else if pA.MatchesPlayed < 30 {
			k = 32.0
		}

		var totalDelta float64

		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			pB := participants[j]

			// Expected score of A vs B
			expectedA := 1.0 / (1.0 + math.Pow(10.0, (pB.OldRating-pA.OldRating)/400.0))

			// Actual outcome S_ab
			var actualA float64
			if pA.Disqualified && !pB.Disqualified {
				actualA = 0.0
			} else if !pA.Disqualified && pB.Disqualified {
				actualA = 1.0
			} else if pA.RankPlace < pB.RankPlace {
				actualA = 1.0
			} else if pA.RankPlace > pB.RankPlace {
				actualA = 0.0
			} else {
				actualA = 0.5
			}

			// Pairwise delta scaled by 1/(N-1)
			pairwiseDelta := (k / float64(n-1)) * (actualA - expectedA)
			totalDelta += pairwiseDelta
		}

		// Disqualified bots get an additional flat penalty of 15 rating points
		if pA.Disqualified {
			totalDelta -= 15.0
		}

		// Round delta to 1 decimal place
		roundedDelta := math.Round(totalDelta*10.0) / 10.0
		newRating := math.Max(100.0, math.Round((pA.OldRating+roundedDelta)*10.0)/10.0)

		results[i] = ParticipantRatingResult{
			AgentVersionID: pA.AgentVersionID,
			OldRating:      pA.OldRating,
			NewRating:      newRating,
			RatingDelta:    roundedDelta,
		}
	}

	return results
}
