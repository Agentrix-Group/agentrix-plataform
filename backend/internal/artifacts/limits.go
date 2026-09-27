package artifacts

import (
	"errors"
	"io"
	"os"
)

const MaxResultBytes int64 = 1 << 20
const MaxReplayBytes int64 = 128 << 20

// ReadBounded returns no partial artifact on quota failure. Stat rejects large
// files before allocation, and the bounded read also detects growth after stat.
func ReadBounded(path string, limit int64) ([]byte, error) {
	if limit < 0 || limit == int64(^uint64(0)>>1) {
		return nil, errors.New("invalid artifact limit")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > limit {
		return nil, errors.New("artifact type or size exceeds quota")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("artifact grew beyond quota")
	}
	return data, nil
}
