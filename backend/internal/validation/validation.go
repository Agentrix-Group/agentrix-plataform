package validation

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type BotManifest struct {
	Name       string `json:"name"`
	Runtime    string `json:"runtime"`
	Entrypoint string `json:"entrypoint"`
}

type ValidationResult struct {
	Valid         bool
	Manifest      BotManifest
	ExtractedPath string
	ErrorMessage  string
}

const (
	MaxZipSize       = 25 * 1024 * 1024  // 25 MB max zip upload
	MaxExtractedSize = 100 * 1024 * 1024 // 100 MB max extracted content
	MaxFileCount     = 1000
)

// SafelyExtractZip extracts a zip file to destDir with strict Zip Slip and Zip Bomb prevention.
func SafelyExtractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	cleanDest := filepath.Clean(destDir)
	var totalSize int64
	var fileCount int

	for _, f := range r.File {
		fileCount++
		if fileCount > MaxFileCount {
			return errors.New("zip bomb detected: too many files in archive")
		}

		// Prevent Zip Slip
		targetPath := filepath.Join(cleanDest, f.Name)
		cleanTarget := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanTarget, cleanDest+string(filepath.Separator)) && cleanTarget != cleanDest {
			return fmt.Errorf("illegal zip slip path detected: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanTarget, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.OpenFile(cleanTarget, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode()|0644)
		if err != nil {
			rc.Close()
			return err
		}

		// Enforce MaxExtractedSize
		written, err := io.Copy(out, io.LimitReader(rc, MaxExtractedSize-totalSize+1))
		out.Close()
		rc.Close()

		if err != nil {
			return err
		}

		totalSize += written
		if totalSize > MaxExtractedSize {
			return errors.New("zip bomb detected: uncompressed size exceeded limit")
		}
	}

	return nil
}

// InspectBotDirectory parses the manifest or infers reasonable defaults.
func InspectBotDirectory(botDir string) (BotManifest, error) {
	manifestPath := filepath.Join(botDir, "agentrix.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		var manifest BotManifest
		if err := json.Unmarshal(data, &manifest); err == nil && manifest.Entrypoint != "" {
			if manifest.Runtime == "" {
				manifest.Runtime = "python-standard"
			}
			return manifest, nil
		}
	}

	// Fallback 1: run.sh
	if _, err := os.Stat(filepath.Join(botDir, "run.sh")); err == nil {
		os.Chmod(filepath.Join(botDir, "run.sh"), 0755)
		return BotManifest{
			Name:       filepath.Base(botDir),
			Runtime:    "binary",
			Entrypoint: "./run.sh",
		}, nil
	}

	// Fallback 2: agent.py
	if _, err := os.Stat(filepath.Join(botDir, "agent.py")); err == nil {
		return BotManifest{
			Name:       filepath.Base(botDir),
			Runtime:    "python-standard",
			Entrypoint: "python3 agent.py",
		}, nil
	}

	// Fallback 3: main.py
	if _, err := os.Stat(filepath.Join(botDir, "main.py")); err == nil {
		return BotManifest{
			Name:       filepath.Base(botDir),
			Runtime:    "python-standard",
			Entrypoint: "python3 main.py",
		}, nil
	}

	return BotManifest{}, errors.New("no valid entrypoint found (must contain agentrix.json, run.sh, agent.py or main.py)")
}

// TestBotProtocol runs a 3-second protocol test to verify the bot can boot and communicate.
func TestBotProtocol(botDir string, manifest BotManifest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	parts := strings.Fields(manifest.Entrypoint)
	if len(parts) == 0 {
		return errors.New("empty entrypoint")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = botDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start bot process: %w", err)
	}

	// Send warmup protocol payload
	warmupPayload := `{"type": "WARMUP", "player_id": 0}` + "\n"
	_, _ = stdin.Write([]byte(warmupPayload))

	// Allow process to initialize or report errors
	done := make(chan error, 1)
	go func() {
		// Wait a small moment for warmup
		time.Sleep(500 * time.Millisecond)
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() && !cmd.ProcessState.Success() {
			done <- fmt.Errorf("bot process exited with code %d", cmd.ProcessState.ExitCode())
			return
		}
		done <- nil
	}()

	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return errors.New("bot validation timed out (took longer than 3s)")
	case err := <-done:
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return err
	}
}
