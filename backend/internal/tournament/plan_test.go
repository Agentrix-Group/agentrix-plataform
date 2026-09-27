package tournament

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCompleteRoundsOneToTwentyTeams(t *testing.T) {
	for n := 1; n <= 20; n++ {
		t.Run(fmt.Sprintf("teams-%d", n), func(t *testing.T) {
			roster := make([]int, n)
			for i := range roster {
				roster[i] = i + 1
			}
			plan, err := New(roster, 42)
			if err != nil {
				t.Fatal(err)
			}
			appear := make([]int, n)
			seats := make([][5]int, n)
			pairs := map[[2]int]int{}
			for i := 0; i < plan.Total; i++ {
				match, err := plan.At(i)
				if err != nil {
					t.Fatal(err)
				}
				seen := map[int]bool{}
				for seat, id := range match.Agents {
					if seen[id] {
						t.Fatal("self encounter")
					}
					seen[id] = true
					appear[id-1]++
					seats[id-1][seat]++
				}
				for a := 0; a < 5; a++ {
					for b := a + 1; b < 5; b++ {
						x, y := match.Agents[a], match.Agents[b]
						if x > y {
							x, y = y, x
						}
						pairs[[2]int{x, y}]++
					}
				}
			}
			for i := range appear {
				if appear[i] != 5*Choose(n-1, 4) {
					t.Fatalf("unequal appearances: %v", appear)
				}
				for _, count := range seats[i] {
					if count != Choose(n-1, 4) {
						t.Fatalf("unequal seats: %v", seats)
					}
				}
			}
			if n >= 5 {
				for a := 1; a <= n; a++ {
					for b := a + 1; b <= n; b++ {
						if pairs[[2]int{a, b}] != 5*Choose(n-2, 3) {
							t.Fatal("unequal rival exposure")
						}
					}
				}
			}
			if n < 5 && plan.Total != 0 {
				t.Fatal("small roster duplicated to fill seats")
			}
			if _, err := plan.At(plan.Total); err == nil {
				t.Fatal("out-of-range match accepted")
			}
		})
	}
}

func TestPlanIsDeterministicAndOwnsRoster(t *testing.T) {
	roster := []int{1, 2, 3, 4, 5, 6}
	a, _ := New(roster, 99)
	b, _ := New(roster, 99)
	roster[0] = 100
	for i := 0; i < a.Total; i++ {
		x, _ := a.At(i)
		y, _ := b.At(i)
		if !reflect.DeepEqual(x, y) {
			t.Fatal("plan changed or depends on caller mutation")
		}
	}
	if _, err := New([]int{1, 1, 2, 3, 4}, 1); err == nil {
		t.Fatal("duplicate roster accepted")
	}
}
