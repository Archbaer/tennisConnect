package saferead

import (
	"errors"
	"io"
)

var ErrTooLarge = errors.New("reader exceeds limit")

// ReadBounded reads at most limit bytes and fails once the reader is larger,
// so a hostile body can never be buffered past the caller's cap.
func ReadBounded(reader io.Reader, limit int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, ErrTooLarge
	}
	return data, nil
}
