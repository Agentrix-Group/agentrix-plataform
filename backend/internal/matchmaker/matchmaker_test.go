package matchmaker

import (
	"agentrix/backend/internal/config"
	"context"
	"sync"
	"testing"
	"time"
)

func doneFor(t *testing.T, m *Matchmaker) <-chan struct{} {
	t.Helper()
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	if m.run == nil {
		t.Fatal("loop not started")
	}
	return m.run.done
}
func await(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("lifecycle did not complete")
	}
}

func TestRestartAndParentCancellation(t *testing.T) {
	m := NewMatchmaker(&config.Config{AutoMatchmaker: true, MatchInterval: 60}, nil, nil)
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		m.Start(ctx)
		if !m.IsActive() {
			t.Fatal("loop inactive")
		}
		done := doneFor(t, m)
		cancel()
		await(t, done)
		if m.IsActive() {
			t.Fatal("canceled loop reported active")
		}
		m.Stop()
		m.Stop()
	}
	m.Start(context.Background())
	m.Stop()
}

func TestConcurrentStartStopIsIdempotent(t *testing.T) {
	m := NewMatchmaker(&config.Config{AutoMatchmaker: true, MatchInterval: 60}, nil, nil)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				m.Start(context.Background())
				m.IsActive()
				m.Stop()
			}
		}()
	}
	wg.Wait()
	m.Stop()
	if m.IsActive() {
		t.Fatal("loop still active")
	}
}

func TestStopCancelsAndWaitsForInFlightScheduling(t *testing.T) {
	m := NewMatchmaker(&config.Config{AutoMatchmaker: true, MatchInterval: 1}, nil, nil)
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	m.poll = func(ctx context.Context) { close(entered); <-ctx.Done(); close(canceled); <-release }
	m.Start(context.Background())
	done := doneFor(t, m)
	await(t, entered)
	stopped := make(chan struct{})
	go func() { m.Stop(); close(stopped) }()
	await(t, canceled)
	m.Start(context.Background()) // Must not replace a stopping generation.
	if doneFor(t, m) != done {
		t.Fatal("overlapping restart")
	}
	select {
	case <-stopped:
		t.Fatal("Stop did not wait for scheduling")
	default:
	}
	close(release)
	await(t, stopped)
	if m.IsActive() {
		t.Fatal("stopped loop reported active")
	}
	m.poll = func(context.Context) {}
	m.Start(context.Background())
	m.Stop()
}

func TestDisabledCanceledAndInvalidIntervalsDoNotStart(t *testing.T) {
	for _, cfg := range []config.Config{{AutoMatchmaker: false, MatchInterval: 1}, {AutoMatchmaker: true, MatchInterval: 0}, {AutoMatchmaker: true, MatchInterval: -1}, {AutoMatchmaker: true, MatchInterval: 86401}} {
		m := NewMatchmaker(&cfg, nil, nil)
		m.Start(context.Background())
		m.Stop()
		if m.IsActive() {
			t.Fatal("invalid loop started")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := NewMatchmaker(&config.Config{AutoMatchmaker: true, MatchInterval: 1}, nil, nil)
	m.Start(ctx)
	if m.IsActive() {
		t.Fatal("started canceled loop")
	}
}
