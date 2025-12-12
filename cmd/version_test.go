package cmd

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// versionErrWriter is a writer that returns an error after writing a configured number of bytes
type versionErrWriter struct {
	err          error
	bytesWritten int
	failAfter    int // fail after this many bytes written (-1 to never fail on Write)
}

func (e *versionErrWriter) Write(p []byte) (n int, err error) {
	if e.failAfter >= 0 && e.bytesWritten >= e.failAfter {
		return 0, e.err
	}
	e.bytesWritten += len(p)
	return len(p), nil
}

// writeVersionOutput simulates the version output writing logic
func writeVersionOutput(w io.Writer, version string) error {
	if _, err := fmt.Fprintf(w, "chamber %s\n", version); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	return nil
}

func TestVersionWriteErrors(t *testing.T) {
	testErr := errors.New("write error")

	tests := []struct {
		name        string
		version     string
		failAfter   int
		expectError bool
	}{
		{
			name:        "write error immediately",
			version:     "v2.0.0",
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "write error with empty version",
			version:     "",
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "no error when writer succeeds",
			version:     "v2.0.0",
			failAfter:   -1,
			expectError: false,
		},
		{
			name:        "no error with long version",
			version:     "v2.0.0-beta.1+build.123",
			failAfter:   -1,
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &versionErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeVersionOutput(w, test.version)

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), testErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVersionOutputSuccess(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{
			name:    "simple version",
			version: "v2.0.0",
		},
		{
			name:    "version with prerelease",
			version: "v2.0.0-beta.1",
		},
		{
			name:    "version with build metadata",
			version: "v2.0.0+build.123",
		},
		{
			name:    "empty version",
			version: "",
		},
		{
			name:    "development version",
			version: "dev",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &versionErrWriter{failAfter: -1}
			err := writeVersionOutput(w, test.version)
			assert.NoError(t, err)
		})
	}
}
