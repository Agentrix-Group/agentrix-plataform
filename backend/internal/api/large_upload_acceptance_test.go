package api

import (
	"agentrix/backend/internal/auth"
	"agentrix/backend/internal/validation"
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReal100MiBUploadAndStorage(t *testing.T) {
	for _, size := range []int64{1024 * 1024, validation.MaxZipSize} {
		t.Run(fmt.Sprintf("%dMiB", size/(1024*1024)), func(t *testing.T) { testRealSizedUpload(t, size) })
	}
}

func testRealSizedUpload(t *testing.T, targetSize int64) {
	if os.Getenv("AGENTRIX_TEST_ACCEPTANCE") != "1" {
		t.Skip("real acceptance opt-in required")
	}
	if os.Getenv("AGENTRIX_TEST_SANDBOX") != "1" || os.Getenv("AGENTRIX_TEST_DATABASE_URL") == "" {
		t.Fatal("real sandbox and PostgreSQL required")
	}
	s, ctx, user, arena, agents := fixtureAPI(t)
	s.cfg.BotsDir = t.TempDir()
	s.cfg.ArbiterPath = os.Getenv("AGENTRIX_TEST_ARBITER_PATH")
	if s.cfg.ArbiterPath == "" {
		t.Fatal("real arbiter required")
	}
	var team int
	if err := s.pool.QueryRow(ctx, "SELECT team_id FROM agent_versions WHERE id=$1", agents[0]).Scan(&team); err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(t.TempDir(), "large-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	source, err := os.ReadFile("../../../bots/heuristic_bot/agent.py")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile("../../../bots/heuristic_bot/agentrix.json")
	if err != nil {
		t.Fatal(err)
	}
	build := func(padding int64) {
		if err := file.Truncate(0); err != nil {
			t.Fatal(err)
		}
		if _, err := file.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		archive := zip.NewWriter(file)
		for _, entry := range []struct {
			Name string
			Data []byte
		}{{"agent.py", source}, {"agentrix.json", manifest}} {
			header := &zip.FileHeader{Name: entry.Name, Method: zip.Store}
			header.SetMode(0644)
			writer, err := archive.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Write(entry.Data); err != nil {
				t.Fatal(err)
			}
		}
		writer, err := archive.CreateHeader(&zip.FileHeader{Name: "weights-padding.bin", Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.CopyN(writer, zeroUpload{}, padding); err != nil {
			t.Fatal(err)
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
	}
	build(0)
	stat, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	paddingSize := targetSize - stat.Size()
	build(paddingSize)
	stat, err = file.Stat()
	if err != nil || stat.Size() != targetSize {
		t.Fatalf("exact ZIP size: %v %v", stat, err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		t.Fatal(err)
	}
	expected := hex.EncodeToString(hash.Sum(nil))
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	var prefix bytes.Buffer
	form := multipart.NewWriter(&prefix)
	for key, value := range map[string]string{"team_id": fmt.Sprint(team), "arena_id": fmt.Sprint(arena)} {
		if err := form.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := form.CreateFormFile("bot_archive", "large.zip"); err != nil {
		t.Fatal(err)
	}
	header := append([]byte(nil), prefix.Bytes()...)
	prefix.Reset()
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	body := io.MultiReader(bytes.NewReader(header), file, bytes.NewReader(prefix.Bytes()))
	token, err := auth.GenerateToken(user, "judge", "admin", s.cfg.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(s.Router())
	defer server.Close()
	client := server.Client()
	client.Timeout = 60 * time.Second
	req, err := http.NewRequest("POST", server.URL+"/api/v1/agents/upload", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 201 {
		t.Fatalf("100 MiB upload: %d %s", response.StatusCode, responseBody)
	}
	var reply struct {
		ID int `json:"agent_id"`
	}
	if err := json.Unmarshal(responseBody, &reply); err != nil {
		t.Fatal(err)
	}
	var path, storedHash, treeHash, status string
	if err := s.pool.QueryRow(ctx, "SELECT artifact_path,sha256,artifact_sha256,status FROM agent_versions WHERE id=$1", reply.ID).Scan(&path, &storedHash, &treeHash, &status); err != nil {
		t.Fatal(err)
	}
	stored, err := os.Open(path + ".zip")
	if err != nil {
		t.Fatal(err)
	}
	defer stored.Close()
	hash.Reset()
	n, err := io.Copy(hash, stored)
	if err != nil || n != targetSize || storedHash != expected || hex.EncodeToString(hash.Sum(nil)) != expected || status != "active" {
		t.Fatal("100 MiB original or DB digest differs")
	}
	actual, err := validation.DigestPackage(path)
	if err != nil || actual != treeHash {
		t.Fatal("100 MiB expanded artifact digest differs")
	}
	if st, err := os.Stat(filepath.Join(path, "weights-padding.bin")); err != nil || st.Size() != paddingSize {
		t.Fatal("large artifact was not extracted")
	}
}
