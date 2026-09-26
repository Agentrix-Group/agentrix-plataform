package ladder

import (
	"math"
	"testing"
)

func TestCalculateMultiplayerElo_ConservationAndRanking(t *testing.T) {
	// 5 bots with equal starting rating (1500)
	input := []ParticipantRatingInput{
		{AgentVersionID: 1, OldRating: 1500, RankPlace: 1, Disqualified: false, MatchesPlayed: 15},
		{AgentVersionID: 2, OldRating: 1500, RankPlace: 2, Disqualified: false, MatchesPlayed: 15},
		{AgentVersionID: 3, OldRating: 1500, RankPlace: 3, Disqualified: false, MatchesPlayed: 15},
		{AgentVersionID: 4, OldRating: 1500, RankPlace: 4, Disqualified: false, MatchesPlayed: 15},
		{AgentVersionID: 5, OldRating: 1500, RankPlace: 5, Disqualified: false, MatchesPlayed: 15},
	}

	results := CalculateMultiplayerElo(input)

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	// 1st place must gain rating
	if results[0].RatingDelta <= 0 {
		t.Errorf("1st place should have positive delta, got %f", results[0].RatingDelta)
	}

	// 5th place must lose rating
	if results[4].RatingDelta >= 0 {
		t.Errorf("5th place should have negative delta, got %f", results[4].RatingDelta)
	}

	// 1st place > 2nd place > 3rd place > 4th place > 5th place
	for i := 0; i < 4; i++ {
		if results[i].RatingDelta <= results[i+1].RatingDelta {
			t.Errorf("participant %d delta (%f) should be greater than %d (%f)",
				i, results[i].RatingDelta, i+1, results[i+1].RatingDelta)
		}
	}

	// Sum of deltas should be approximately 0 (conservation)
	var sumDelta float64
	for _, r := range results {
		sumDelta += r.RatingDelta
	}
	if math.Abs(sumDelta) > 0.5 {
		t.Errorf("expected sum of deltas near 0, got %f", sumDelta)
	}
}

func TestCalculateMultiplayerElo_DisqualificationPenalty(t *testing.T) {
	input := []ParticipantRatingInput{
		{AgentVersionID: 1, OldRating: 1500, RankPlace: 1, Disqualified: false, MatchesPlayed: 10},
		{AgentVersionID: 2, OldRating: 1500, RankPlace: 2, Disqualified: false, MatchesPlayed: 10},
		{AgentVersionID: 3, OldRating: 1500, RankPlace: 3, Disqualified: false, MatchesPlayed: 10},
		{AgentVersionID: 4, OldRating: 1500, RankPlace: 4, Disqualified: false, MatchesPlayed: 10},
		{AgentVersionID: 5, OldRating: 1500, RankPlace: 5, Disqualified: true, MatchesPlayed: 10},
	}

	results := CalculateMultiplayerElo(input)

	// Disqualified bot must suffer strong penalty
	if results[4].RatingDelta > -25.0 {
		t.Errorf("disqualified bot should have delta < -25, got %f", results[4].RatingDelta)
	}
}
