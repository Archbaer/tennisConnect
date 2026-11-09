package saferead

import (
	"errors"
	"strings"
	"testing"
)

func TestReadBoundedRejectsOversizedReader(t *testing.T) {
	const limit = 8
	exact := strings.Repeat("a", limit)
	data, err := ReadBounded(strings.NewReader(exact), limit)
	if err != nil || len(data) != limit {
		t.Fatalf("ReadBounded at limit = %d bytes, err %v", len(data), err)
	}
	if _, err := ReadBounded(strings.NewReader(exact+"a"), limit); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("ReadBounded over limit err = %v, want ErrTooLarge", err)
	}
}
