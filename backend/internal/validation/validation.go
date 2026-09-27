package validation

import (
	"archive/zip"
	"bytes"
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

	"agentrix/backend/internal/sandbox"
)

type BotManifest struct {
	Version         int    `json:"version"`
	ProtocolVersion int    `json:"protocol_version"`
	Name            string `json:"name"`
	Runtime         string `json:"runtime"`
	Entrypoint      string `json:"entrypoint"`
}

type ValidationResult struct {
	Valid         bool
	Manifest      BotManifest
	ExtractedPath string
	ErrorMessage  string
}

const (
	MaxZipSize       = 100 * 1024 * 1024 // 100 MiB of compressed package bytes
	MaxExtractedSize = 512 * 1024 * 1024 // 512 MiB of expanded package bytes
	MaxManifestSize  = 16 * 1024
	MaxFileCount     = 1000
)

// SafelyExtractZip extracts a zip file to destDir with strict Zip Slip and Zip Bomb prevention.
func SafelyExtractZip(zipPath, destDir string) error {
	st, err := os.Stat(zipPath)
	if err != nil {
		return err
	}
	if st.Size() > MaxZipSize {
		return errors.New("compressed archive exceeds upload limit")
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()
	if err := validateArchive(r.File); err != nil {
		return err
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	root, err := os.Lstat(destDir)
	if err != nil || !root.IsDir() || root.Mode()&os.ModeSymlink != 0 {
		return errors.New("extraction destination must be a real directory")
	}
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("extraction destination must be empty")
	}

	cleanDest := filepath.Clean(destDir)
	var totalSize int64

	for _, f := range r.File {
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

		out, err := os.OpenFile(cleanTarget, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644|(f.Mode().Perm()&0111))
		if err != nil {
			rc.Close()
			return err
		}

		// Enforce MaxExtractedSize
		written, err := io.Copy(out, io.LimitReader(rc, MaxExtractedSize-totalSize+1))
		closeErr := out.Close()
		rc.Close()

		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}

		totalSize += written
		if totalSize > MaxExtractedSize {
			return errors.New("zip bomb detected: uncompressed size exceeded limit")
		}
	}

	return nil
}

// Preflight every central-directory entry before writing any expanded bytes.
func validateArchive(files []*zip.File) error {
	if len(files) == 0 || len(files) > MaxFileCount {
		return errors.New("archive entry count outside quota")
	}
	seen := map[string]bool{}
	nodes := map[string]bool{}
	var total uint64
	for _, f := range files {
		name := strings.TrimSuffix(f.Name, "/")
		if name == "" || len(name) > 1024 || strings.ContainsAny(name, "\\\x00") || filepath.IsAbs(name) || filepath.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") {
			return fmt.Errorf("noncanonical archive path: %q", name)
		}
		if seen[name] {
			return fmt.Errorf("duplicate archive path: %q", name)
		}
		seen[name] = true
		for node := name; node != "."; node = filepath.Dir(node) {
			nodes[node] = true
			if len(nodes) > MaxFileCount {
				return errors.New("archive paths exceed expanded entry quota")
			}
		}
		if !f.FileInfo().IsDir() && !f.Mode().IsRegular() {
			return errors.New("archive contains special file")
		}
		if f.UncompressedSize64 > MaxExtractedSize-total {
			return errors.New("declared expanded package exceeds quota")
		}
		total += f.UncompressedSize64
		if f.UncompressedSize64 > 1024*1024 && f.UncompressedSize64/(f.CompressedSize64+1) > 200 {
			return errors.New("archive compression ratio exceeds quota")
		}
	}
	return nil
}

// InspectBotDirectory requires the versioned package contract; no inferred
// commands, runtimes, or protocol versions may differ between admission/matches.
func InspectBotDirectory(botDir string) (BotManifest, error) {
	manifestPath := filepath.Join(botDir, "agentrix.json")
	f, err := os.Open(manifestPath)
	if err != nil {
		return BotManifest{}, errors.New("agentrix.json is required")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxManifestSize+1))
	if err != nil {
		return BotManifest{}, err
	}
	if len(data) > MaxManifestSize {
		return BotManifest{}, errors.New("manifest exceeds quota")
	}
	var manifest BotManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil {
		return BotManifest{}, errors.New("invalid agentrix.json")
	}
	var trailing interface{}
	if decoder.Decode(&trailing) != io.EOF {
		return BotManifest{}, errors.New("trailing manifest content")
	}
	if manifest.Version != 1 || manifest.ProtocolVersion != 1 || strings.TrimSpace(manifest.Name) == "" || len(manifest.Name) > 128 {
		return BotManifest{}, errors.New("manifest version, protocol_version and name required")
	}
	switch manifest.Runtime {
	case "python-standard", "python-onnx", "binary":
	default:
		return BotManifest{}, errors.New("unsupported manifest runtime")
	}
	if err := validateEntrypoint(botDir, manifest.Entrypoint); err != nil {
		return BotManifest{}, err
	}
	if manifest.Runtime != "binary" && !strings.HasPrefix(manifest.Entrypoint, "python3 ") {
		return BotManifest{}, errors.New("Python runtime requires python3 script entrypoint")
	}
	return manifest, nil
}

func validateEntrypoint(botDir, entrypoint string) error {
	parts := strings.Fields(entrypoint)
	if len(parts) == 0 || len(parts) > 2 {
		return errors.New("entrypoint must be a local executable or python3 script")
	}
	file := parts[0]
	if len(parts) == 1 && !strings.HasPrefix(file, "./") {
		return errors.New("executable entrypoint must start with ./")
	}
	if len(parts) == 2 {
		if parts[0] != "python3" {
			return errors.New("only python3 accepts a script argument")
		}
		file = parts[1]
	}
	file = strings.TrimPrefix(file, "./")
	if filepath.IsAbs(file) || filepath.Clean(file) != file || file == ".." || strings.HasPrefix(file, "../") {
		return errors.New("entrypoint must stay inside the package")
	}
	st, err := os.Lstat(filepath.Join(botDir, file))
	if err != nil || !st.Mode().IsRegular() {
		return errors.New("entrypoint must be a regular package file")
	}
	if len(parts) == 1 && st.Mode().Perm()&0111 == 0 {
		return errors.New("executable entrypoint must have execute permission")
	}
	return nil
}

// TestBotProtocol executes the actual arbiter admission mode with the same
// sandbox launcher used for matches. Missing infrastructure fails closed.
func TestBotProtocol(botDir string, manifest BotManifest, arbiterPath string) error {
	if err := validateEntrypoint(botDir, manifest.Entrypoint); err != nil {
		return err
	}
	parts, err := sandbox.Command(botDir, manifest.Entrypoint)
	if err != nil {
		return err
	}
	defer sandbox.Stop(parts)
	for i, arg := range parts {
		parts[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	command := strings.Join(parts, " ")
	arbiter, err := filepath.Abs(arbiterPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, arbiter, "--admit", "--b0", command)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "XDG_RUNTIME_DIR=" + os.Getenv("XDG_RUNTIME_DIR")}
	var output bytes.Buffer
	cmd.Stdout = &output
	// Participant stderr is discarded by the arbiter, never mixed with JSON.
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("arbiter admission failed: %w", err)
	}
	var receipt struct {
		Status          string `json:"status"`
		ProtocolVersion int    `json:"protocol_version"`
		ValidatedTicks  int    `json:"validated_ticks"`
	}
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil || receipt.Status != "ADMITTED" || receipt.ProtocolVersion != 1 || receipt.ValidatedTicks != 3 {
		return errors.New("invalid arbiter admission receipt")
	}
	return nil
}
