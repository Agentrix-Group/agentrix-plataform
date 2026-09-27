package matchmaker

import (
	"context"
	"log"
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
	lifecycle  sync.Mutex
	run        *loopRun
	poll       func(context.Context)
	activeLock sync.Mutex
}

type loopRun struct {
	cancel   context.CancelFunc
	done     chan struct{}
	stopping bool
}

func NewMatchmaker(cfg *config.Config, pool *db.Pool, r *runner.MatchRunner) *Matchmaker {
	m := &Matchmaker{
		cfg:    cfg,
		pool:   pool,
		runner: r,
	}
	m.poll = m.pollAndSchedule
	return m
}

func (m *Matchmaker) Start(ctx context.Context) {
	if !m.cfg.AutoMatchmaker {
		log.Println("[MATCHMAKER] Continuous auto-matchmaker is disabled by configuration")
		return
	}

	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	if m.run != nil || ctx.Err() != nil {
		return
	}
	if m.cfg.MatchInterval <= 0 || m.cfg.MatchInterval > 86400 {
		log.Println("[MATCHMAKER] Refusing scheduling interval outside 1..86400 seconds")
		return
	}
	loopCtx, cancel := context.WithCancel(ctx)
	run := &loopRun{cancel: cancel, done: make(chan struct{})}
	m.run = run
	log.Printf("[MATCHMAKER] Starting continuous ladder matchmaker loop (interval: %ds)...", m.cfg.MatchInterval)

	go func() {
		defer func() {
			cancel()
			m.lifecycle.Lock()
			m.run = nil
			close(run.done)
			m.lifecycle.Unlock()
		}()
		ticker := time.NewTicker(time.Duration(m.cfg.MatchInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-loopCtx.Done():
				log.Println("[MATCHMAKER] Stopping matchmaker loop...")
				return
			case <-ticker.C:
				if loopCtx.Err() == nil {
					m.poll(loopCtx)
				}
			}
		}
	}()
}

func (m *Matchmaker) Stop() {
	m.lifecycle.Lock()
	run := m.run
	if run != nil {
		run.stopping = true
		run.cancel()
	}
	m.lifecycle.Unlock()
	if run != nil {
		<-run.done
	}
}

func (m *Matchmaker) IsActive() bool {
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	return m.run != nil && !m.run.stopping
}

func (m *Matchmaker) pollAndSchedule(ctx context.Context) {
	m.activeLock.Lock()
	defer m.activeLock.Unlock()
	rows, err := m.pool.Query(ctx, "SELECT id FROM arenas WHERE is_active=true ORDER BY id")
	if err != nil {
		log.Printf("[MATCHMAKER] arenas: %v", err)
		return
	}
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		log.Printf("[MATCHMAKER] arenas: %v", err)
		return
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		if err := m.runner.ScheduleAutomaticRound(ctx, id); err != nil {
			log.Printf("[MATCHMAKER] round in arena %d: %v", id, err)
		}
	}
}
