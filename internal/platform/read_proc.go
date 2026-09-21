package platform

import (
	"io"
	"os"
	"sync"
)

var procBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 32*1024)
		return &buf
	},
}

// readProcFile reads a file in /proc without stat-ing it, using a pooled buffer.
func readProcFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bufPtr := procBufPool.Get().(*[]byte)
	buf := *bufPtr
	defer procBufPool.Put(bufPtr)

	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}

	if err == nil {
		// The buffer is full. There might be more data. Let's read the rest.
		rest, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		res := make([]byte, n+len(rest))
		copy(res, buf[:n])
		copy(res[n:], rest)
		return res, nil
	}

	res := make([]byte, n)
	copy(res, buf[:n])
	return res, nil
}
