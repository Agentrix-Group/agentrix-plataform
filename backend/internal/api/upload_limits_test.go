package api

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"agentrix/backend/internal/auth"
	"agentrix/backend/internal/config"
	"agentrix/backend/internal/validation"
)

type zeroUpload struct{}

func (zeroUpload) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestUploadRejectsMultipleFilesAndSizeBeforeStorage(t *testing.T) {
	s := NewServer(&config.Config{JWTSecret: "test-secret"}, nil, nil, nil)
	token, err := auth.GenerateToken(1, "user", "player", s.cfg.JWTSecret)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		size      int64
		extraFile bool
		want      int
	}{
		{"zip-over-limit", validation.MaxZipSize + 1, false, 413},
		{"http-over-limit", validation.MaxZipSize + 2*1024*1024, false, 413},
		{"two-zips", 0, true, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var prefix bytes.Buffer
			writer := multipart.NewWriter(&prefix)
			part, err := writer.CreateFormFile("bot_archive", "bot.zip")
			if err != nil {
				t.Fatal(err)
			}
			if tc.extraFile {
				if _, err := part.Write([]byte("zip")); err != nil {
					t.Fatal(err)
				}
				if _, err := writer.CreateFormFile("bot_archive", "other.zip"); err != nil {
					t.Fatal(err)
				}
			}
			header := append([]byte(nil), prefix.Bytes()...)
			prefix.Reset()
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			body := io.MultiReader(bytes.NewReader(header), io.LimitReader(zeroUpload{}, tc.size), bytes.NewReader(prefix.Bytes()))
			r := httptest.NewRequest("POST", "/api/v1/agents/upload", body)
			r.Header.Set("Content-Type", writer.FormDataContentType())
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			s.Router().ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}
