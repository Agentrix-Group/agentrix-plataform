package api

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
)

// Validate the entire compressed stream before writing HTTP headers. Never
// reinterpret corrupt gzip as raw JSON. The second pass uses the same file.
func replayStream(ctx context.Context, f *os.File, limit int64) (io.Reader, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if limit < 0 || limit > int64(^uint64(0)>>1)-(1<<20) {
		return nil, nil, errors.New("invalid replay quota")
	}
	st, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > limit+(1<<20) {
		return nil, nil, errors.New("stored replay exceeds quota")
	}
	var magic [2]byte
	_, err = f.ReadAt(magic[:], 0)
	if err != nil && err != io.EOF {
		return nil, nil, err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}
	if magic != [2]byte{0x1f, 0x8b} {
		if st.Size() > limit {
			return nil, nil, errors.New("raw replay exceeds quota")
		}
		return cancelableReader{ctx, io.LimitReader(f, limit)}, func() {}, nil
	}
	reader, err := gzip.NewReader(cancelableReader{ctx, f})
	if err != nil {
		return nil, nil, err
	}
	count, err := io.Copy(io.Discard, cancelableReader{ctx, io.LimitReader(reader, limit+1)})
	reader.Close()
	if err != nil {
		return nil, nil, err
	}
	if count > limit {
		return nil, nil, errors.New("expanded replay exceeds quota")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}
	reader, err = gzip.NewReader(cancelableReader{ctx, f})
	if err != nil {
		return nil, nil, err
	}
	return cancelableReader{ctx, io.LimitReader(reader, limit)}, func() { reader.Close() }, nil
}

type cancelableReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r cancelableReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(data)
	if canceled := r.ctx.Err(); canceled != nil {
		return 0, canceled
	}
	return n, err
}
