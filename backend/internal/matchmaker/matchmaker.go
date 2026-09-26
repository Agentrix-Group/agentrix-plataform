package matchmaker

import (
	"context"
	"log"
	"math/rand"
	"sort"
	"sync"
	"time"

	"agentrix/backend/internal/config"
	"agentrix/backend/internal/db"
	"agentrix/backend/internal/runner"
)

type Matchmaker struct {
	cfg        *config.Config
	pool       *db.Pool
	runner     *runner.MatchRunner
	stopCh     chan struct{}
	wg         sync.WaitGroup
	activeLock sync.Mutex
	isRunning  bool
}

func NewMatchmaker(cfg *config.Config, pool *db.Pool, r *runner.MatchRunner) *Matchmaker {
	return &Matchmaker{
		cfg:    cfg,
		pool:   pool,
		runner: r,
		stopCh: make(chan struct{}),
	}
}

func (m *Matchmaker) Start(ctx context.Context) {
	if !m.cfg.AutoMatchmaker {
		log.Println("[MATCHMAKER] Continuous auto-matchmaker is disabled by configuration")
		return
	}

	log.Printf("[MATCHMAKER] Starting continuous ladder matchmaker loop (interval: %ds)...", m.cfg.MatchInterval)
	m.isRunning = true
	m.wg.Add(1)

	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(time.Duration(m.cfg.MatchInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-m.stopCh:
				log.Println("[MATCHMAKER] Stopping matchmaker loop...")
				return
			case <-ticker.C:
				m.pollAndSchedule(ctx)
			}
		}
	}()
}

func (m *Matchmaker) Stop() {
	if m.isRunning {
		close(m.stopCh)
		m.wg.Wait()
		m.isRunning = false
	}
}

func (m *Matchmaker) IsActive() bool {
	return m.isRunning
}

type botCandidate struct {
	ID            int
	Rating        int
	MatchesPlayed int
	LastMatchAt   *time.Time
}

func (m *Matchmaker) pollAndSchedule(ctx context.Context) {
	m.activeLock.Lock()
	defer m.activeLock.Unlock()

	// 1. Fetch active arenas
	rows, err := m.pool.Query(ctx, "SELECT id FROM arenas WHERE is_active = true")
	if err != nil {
		log.Printf("[MATCHMAKER] Error querying arenas: %v", err)
		return
	}
	defer rows.Close()

	var arenaIDs []int
	for rows.Next() {
		var aid int
		if err := rows.Scan(&aid); err == nil {
			arenaIDs = append(arenaIDs, aid)
		}
	}
	rows.Close()

	for _, arenaID := range arenaIDs {
		// Fetch eligible active bots for this arena
		botRows, err := m.pool.Query(ctx, `
			SELECT av.id, COALESCE(le.display_rating, 1500), COALESCE(le.matches_played, 0), le.last_match_at
			FROM agent_versions av
			LEFT JOIN ladder_entries le ON le.arena_id = av.arena_id AND le.agent_version_id = av.id
			WHERE av.arena_id = $1 AND av.status = 'active'
			ORDER BY le.last_match_at ASC NULLS FIRST
		`, arenaID)
		if err != nil {
			continue
		}

		var candidates []botCandidate
		for botRows.Next() {
			var b botCandidate
			if err := botRows.Scan(&b.ID, &b.Rating, &b.MatchesPlayed, &b.LastMatchAt); err == nil {
				candidates = append(candidates, b)
			}
		}
		botRows.Close()

		if len(candidates) < 5 {
			continue // need at least 5 active bots to form a match
		}

		// Pick the bot that has been waiting the longest as anchor (candidates[0])
		anchor := candidates[0]
		remaining := candidates[1:]

		// Sort remaining bots by closeness to anchor rating
		type distanceCandidate struct {
			b    botCandidate
			dist int
		}
		var distList []distanceCandidate
		for _, b := range remaining {
			d := b.Rating - anchor.Rating
			if d < 0 {
				d = -d
			}
			distList = append(distList, distanceCandidate{b: b, dist: d})
		}

		sort.Slice(distList, func(i, j int) bool {
			return distList[i].dist < distList[j].dist
		})

		selectedIDs := []int{anchor.ID}
		for i := 0; i < 4 && i < len(distList); i++ {
			selectedIDs = append(selectedIDs, distList[i].b.ID)
		}

		if len(selectedIDs) == 5 {
			// Randomize seat allocation
			rand.Shuffle(len(selectedIDs), func(i, j int) {
				selectedIDs[i], selectedIDs[j] = selectedIDs[j], selectedIDs[i]
			})

			matchID, err := m.runner.ScheduleMatch(ctx, arenaID, selectedIDs, nil)
			if err != nil {
				log.Printf("[MATCHMAKER] Failed to schedule match: %v", err)
				continue
			}

			log.Printf("[MATCHMAKER] Auto-scheduled continuous ladder match #%d in arena #%d", matchID, arenaID)

			// Execute asynchronously
			go func(mid int) {
				execCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				if err := m.runner.ExecuteMatch(execCtx, mid); err != nil {
					log.Printf("[MATCHMAKER] Match #%d execution returned error: %v", mid, err)
				}
			}(matchID)
		}
	}
}
