package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestReplayStreamBoundariesCorruptionAndExpansion(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	writer.Write([]byte("1234"))
	writer.Close()
	for _, tc := range []struct {
		name  string
		data  []byte
		limit int64
		valid bool
	}{
		{"raw boundary", []byte("1234"), 4, true}, {"raw oversized", []byte("1234"), 3, false},
		{"gzip boundary", compressed.Bytes(), 4, true}, {"gzip expanded overlimit", compressed.Bytes(), 3, false},
		{"truncated gzip", compressed.Bytes()[:compressed.Len()-4], 4, false},
		{"bad gzip header", []byte{0x1f, 0x8b, 0}, 4, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "replay")
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			stream, closeStream, err := replayStream(context.Background(), f, tc.limit)
			if !tc.valid {
				if err == nil {
					closeStream()
					t.Fatal("unsafe replay accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer closeStream()
			data, err := io.ReadAll(stream)
			if err != nil || string(data) != "1234" {
				t.Fatalf("%q %v", data, err)
			}
		})
	}
}

func TestReplayStreamCancellationBeforeValidationAndServing(t *testing.T) {
	var data bytes.Buffer
	writer := gzip.NewWriter(&data)
	writer.Write([]byte("fixture"))
	writer.Close()
	path := filepath.Join(t.TempDir(), "replay.gz")
	if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := replayStream(ctx, f, 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled validation: %v", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	stream, closeStream, err := replayStream(ctx, f, 100)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStream()
	cancel()
	served, err := io.ReadAll(stream)
	if !errors.Is(err, context.Canceled) || len(served) != 0 {
		t.Fatalf("served after cancellation: %q %v", served, err)
	}
}

func TestReplayCapacityReturns429AndReleasesSlot(t *testing.T) {
	s, ctx, _, arena, _ := fixtureAPI(t)
	s.cfg.ReplaysDir = t.TempDir()
	var id int
	if err := s.pool.QueryRow(ctx, "INSERT INTO matches(arena_id,seed,status) VALUES($1,1,'finished') RETURNING id", arena).Scan(&id); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.cfg.ReplaysDir, "fixture.json")
	if err := os.WriteFile(path, []byte(`{"fixture":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, "INSERT INTO replays(match_id,file_path,sha256) VALUES($1,$2,'fixture')", id, path); err != nil {
		t.Fatal(err)
	}
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/v1/matches/%d/replay", id), nil))
		return w
	}
	s.replaySlots <- struct{}{}
	s.replaySlots <- struct{}{}
	w := request()
	if w.Code != 429 || w.Header().Get("Retry-After") != "1" {
		t.Fatalf("busy replay: %d %s", w.Code, w.Body.String())
	}
	<-s.replaySlots
	w = request()
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if len(s.replaySlots) != 1 {
		t.Fatal("request did not release its slot")
	}
	if err := os.WriteFile(path, []byte{0x1f, 0x8b, 0}, 0600); err != nil {
		t.Fatal(err)
	}
	w = request()
	if w.Code != 500 || len(s.replaySlots) != 1 {
		t.Fatal("corrupt replay leaked a capacity slot")
	}
	<-s.replaySlots
}
