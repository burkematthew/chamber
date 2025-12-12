package cmd

import (
	"errors"
	"fmt"
	"io"
	"testing"
	"text/tabwriter"

	"github.com/stretchr/testify/assert"
)

// tagReadErrWriter is a writer that returns an error after writing a configured number of bytes
type tagReadErrWriter struct {
	err          error
	bytesWritten int
	failAfter    int // fail after this many bytes written (-1 to never fail on Write)
}

func (e *tagReadErrWriter) Write(p []byte) (n int, err error) {
	if e.failAfter >= 0 && e.bytesWritten >= e.failAfter {
		return 0, e.err
	}
	e.bytesWritten += len(p)
	return len(p), nil
}

// writeTagReadQuietOutput simulates the quiet mode output from the tagRead function
func writeTagReadQuietOutput(w io.Writer, tags map[string]string) error {
	if _, err := fmt.Fprintf(w, "%s\n", tags); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

// writeTagReadOutput simulates the normal output writing logic from the tagRead function
func writeTagReadOutput(w io.Writer, tags map[string]string) (returnErr error) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, '\t', 0)
	defer func() {
		if err := tw.Flush(); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("failed to flush output: %w", err)
		}
	}()

	if _, err := fmt.Fprintln(tw, "Key\tValue"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	for k, v := range tags {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", k, v); err != nil {
			return fmt.Errorf("failed to write tag: %w", err)
		}
	}
	return nil
}

func TestTagReadQuietWriteErrors(t *testing.T) {
	testErr := errors.New("write error")

	tests := []struct {
		name        string
		tags        map[string]string
		failAfter   int
		expectError bool
	}{
		{
			name:        "write error in quiet mode with empty tags",
			tags:        map[string]string{},
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "write error in quiet mode with tags",
			tags:        map[string]string{"env": "prod", "team": "platform"},
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "no error when writer succeeds",
			tags:        map[string]string{"env": "prod"},
			failAfter:   -1,
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &tagReadErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeTagReadQuietOutput(w, test.tags)

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), testErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTagReadWriteErrors(t *testing.T) {
	testErr := errors.New("write error")

	tests := []struct {
		name        string
		tags        map[string]string
		failAfter   int
		expectError bool
	}{
		{
			name:        "write error with empty tags",
			tags:        map[string]string{},
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "write error with single tag",
			tags:        map[string]string{"environment": "production"},
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "write error with multiple tags",
			tags:        map[string]string{"env": "prod", "team": "platform", "app": "chamber"},
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "no error when writer succeeds with empty tags",
			tags:        map[string]string{},
			failAfter:   -1,
			expectError: false,
		},
		{
			name:        "no error when writer succeeds with tags",
			tags:        map[string]string{"env": "prod", "team": "platform"},
			failAfter:   -1,
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &tagReadErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeTagReadOutput(w, test.tags)

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), testErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTagReadFlushError(t *testing.T) {
	tests := []struct {
		name      string
		tags      map[string]string
		failAfter int
	}{
		{
			name:      "flush error with empty tags",
			tags:      map[string]string{},
			failAfter: 5,
		},
		{
			name:      "flush error with single tag",
			tags:      map[string]string{"env": "prod"},
			failAfter: 15,
		},
		{
			name:      "flush error with multiple tags",
			tags:      map[string]string{"env": "prod", "team": "platform", "app": "chamber"},
			failAfter: 30,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testErr := errors.New("io error")
			w := &tagReadErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeTagReadOutput(w, test.tags)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), testErr.Error())
		})
	}
}

func TestTagReadOutputSuccess(t *testing.T) {
	tests := []struct {
		name string
		tags map[string]string
	}{
		{
			name: "empty tags",
			tags: map[string]string{},
		},
		{
			name: "single tag",
			tags: map[string]string{"environment": "production"},
		},
		{
			name: "multiple tags",
			tags: map[string]string{
				"environment": "production",
				"team":        "platform",
				"application": "chamber",
			},
		},
		{
			name: "tags with special characters in values",
			tags: map[string]string{
				"url":  "https://example.com/path?query=1",
				"path": "/var/log/app.log",
			},
		},
		{
			name: "aws-style tags",
			tags: map[string]string{
				"aws:cloudformation:stack-name": "my-stack",
				"aws:cloudformation:stack-id":   "arn:aws:cloudformation:us-east-1:123456789:stack/my-stack/guid",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &tagReadErrWriter{failAfter: -1}
			err := writeTagReadOutput(w, test.tags)
			assert.NoError(t, err)
		})
	}
}

func TestTagReadQuietOutputSuccess(t *testing.T) {
	tests := []struct {
		name string
		tags map[string]string
	}{
		{
			name: "empty tags",
			tags: map[string]string{},
		},
		{
			name: "single tag",
			tags: map[string]string{"env": "prod"},
		},
		{
			name: "multiple tags",
			tags: map[string]string{"env": "prod", "team": "platform"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &tagReadErrWriter{failAfter: -1}
			err := writeTagReadQuietOutput(w, test.tags)
			assert.NoError(t, err)
		})
	}
}
