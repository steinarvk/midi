package limitreader

import "io"

type limitReader struct {
	underlying io.Reader

	totalBytesRead int64
	readLimit      int64
}

func New(r io.Reader, n int64) io.Reader {
	return &limitReader{
		underlying: r,
		readLimit:  n,
	}
}

func (r *limitReader) Read(buf []byte) (int, error) {
	remaining := r.readLimit - r.totalBytesRead
	if remaining <= 0 {
		return 0, io.EOF
	}

	var n int
	var err error

	if int64(len(buf)) > remaining {
		n, err = r.underlying.Read(buf[:remaining])
	} else {
		n, err = r.underlying.Read(buf)
	}

	r.totalBytesRead += int64(n)

	return n, err
}
