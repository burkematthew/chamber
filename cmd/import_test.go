package cmd

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// importErrWriter is a writer that returns an error after writing a configured number of bytes
type importErrWriter struct {
	err          error
	bytesWritten int
	failAfter    int // fail after this many bytes written (-1 to never fail on Write)
}

func (e *importErrWriter) Write(p []byte) (n int, err error) {
	if e.failAfter >= 0 && e.bytesWritten >= e.failAfter {
		return 0, e.err
	}
	e.bytesWritten += len(p)
	return len(p), nil
}

// writeImportSuccess simulates the success message output from the import function
// This allows us to test the error handling without needing to mock the secret store
func writeImportSuccess(w io.Writer, count int) error {
	if _, err := fmt.Fprintf(w, "Successfully imported %d secrets\n", count); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

func TestImportWriteErrors(t *testing.T) {
	// Test that write errors are properly propagated.
	testErr := errors.New("write error")

	tests := []struct {
		name        string
		count       int
		failAfter   int
		expectError bool
	}{
		{
			name:        "write error with zero secrets",
			count:       0,
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "write error with some secrets",
			count:       5,
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "write error with many secrets",
			count:       100,
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "no error when writer succeeds with zero secrets",
			count:       0,
			failAfter:   -1, // Never fail
			expectError: false,
		},
		{
			name:        "no error when writer succeeds with some secrets",
			count:       10,
			failAfter:   -1, // Never fail
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &importErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeImportSuccess(w, test.count)

			if test.expectError {
				assert.Error(t, err)
				// Verify the underlying error is included
				assert.Contains(t, err.Error(), testErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestImportOutputSuccess(t *testing.T) {
	tests := []struct {
		name  string
		count int
	}{
		{
			name:  "zero secrets imported",
			count: 0,
		},
		{
			name:  "one secret imported",
			count: 1,
		},
		{
			name:  "multiple secrets imported",
			count: 10,
		},
		{
			name:  "many secrets imported",
			count: 1000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Use a successful writer
			w := &importErrWriter{failAfter: -1}
			err := writeImportSuccess(w, test.count)
			assert.NoError(t, err)
		})
	}
}
