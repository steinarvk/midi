package contextreader

import (
	"fmt"
	"io"
)

const (
	numberOfContextBytes = 64
)

type ContextReader struct {
	underlying io.Reader

	totalBytesRead int64
	lastBytesRead  []byte
}

func (r *ContextReader) Read(buf []byte) (int, error) {
	n, err := r.underlying.Read(buf)

	r.totalBytesRead += int64(n)
	if n > 0 {
		r.lastBytesRead = append(r.lastBytesRead, buf[:n]...)
	}
	if len(r.lastBytesRead) > numberOfContextBytes {
		r.lastBytesRead = r.lastBytesRead[len(r.lastBytesRead)-numberOfContextBytes:]
	}

	return n, err
}

func (r *ContextReader) WrapError(err error) error {
	return fmt.Errorf("after %d bytes (last: % 02x): %v", r.totalBytesRead, r.lastBytesRead, err)
}

func New(r io.Reader) *ContextReader {
	return &ContextReader{
		underlying: r,
	}
}
